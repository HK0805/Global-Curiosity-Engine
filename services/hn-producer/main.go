package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/HK0805/Global-Curiosity-Engine/internal/config"
	internalkafka "github.com/HK0805/Global-Curiosity-Engine/internal/kafka"
	"github.com/HK0805/Global-Curiosity-Engine/internal/logging"
	kafkago "github.com/segmentio/kafka-go"
)

const (
	serviceName            = "hn-producer"
	topicName              = "raw.hn.stories"
	newStoriesURL          = "https://hacker-news.firebaseio.com/v0/newstories.json"
	itemURLTemplate        = "https://hacker-news.firebaseio.com/v0/item/%d.json"
	pollInterval           = 30 * time.Second
	maxIDsPerPoll          = 25
	maxResponseBodyBytes   = 2 * 1024 * 1024
	recentIDCapacity       = 5000
	maxConsecutiveFailures = 5
)

type hnStory struct {
	ID      int64  `json:"id"`
	Type    string `json:"type"`
	Deleted bool   `json:"deleted"`
	Dead    bool   `json:"dead"`
	Title   string `json:"title"`
	URL     string `json:"url"`
}

func main() {
	cfg := config.Load()
	logger := logging.New(serviceName)
	writer := internalkafka.NewWriter(cfg, topicName)
	defer func() {
		if err := writer.Close(); err != nil {
			logger.Error("failed to close kafka writer", "error", err)
		}
	}()

	logger.Info(
		"service starting",
		"kafka_broker", cfg.KafkaBroker,
		"topic", topicName,
		"new_stories_url", newStoriesURL,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	producer := newHNProducer(writer, logger)

	if err := producer.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("service stopped with error", "error", err)
	}

	logger.Info("service shutting down")
}

type hnProducer struct {
	client *http.Client
	writer *kafkago.Writer
	logger *slog.Logger
	seen   *recentHNIDSet
}

func newHNProducer(writer *kafkago.Writer, logger *slog.Logger) *hnProducer {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}

	return &hnProducer{
		client: &http.Client{
			Transport: transport,
			Timeout:   20 * time.Second,
		},
		writer: writer,
		logger: logger,
		seen:   newRecentHNIDSet(recentIDCapacity),
	}
}

func (p *hnProducer) Run(ctx context.Context) error {
	p.logger.Info("poller ready", "poll_interval", pollInterval.String(), "max_ids_per_poll", maxIDsPerPoll)

	consecutiveFailures := 0
	backoff := time.Second

	if err := p.pollOnce(ctx); err != nil {
		consecutiveFailures++
		p.logger.Warn("initial hn poll failed", "error", err)
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			err := p.pollOnce(ctx)
			if err == nil {
				consecutiveFailures = 0
				backoff = time.Second
				continue
			}

			consecutiveFailures++
			p.logger.Warn("hn poll failed", "error", err, "consecutive_failures", consecutiveFailures)

			if consecutiveFailures < maxConsecutiveFailures {
				continue
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}

			if backoff < 30*time.Second {
				backoff *= 2
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
			}
		}
	}
}

func (p *hnProducer) pollOnce(ctx context.Context) error {
	ids, err := p.fetchNewStoryIDs(ctx)
	if err != nil {
		return err
	}

	published := 0
	processed := 0

	for _, id := range ids {
		if id <= 0 || p.seen.Contains(id) {
			continue
		}

		rawStory, storyID, err := p.fetchStory(ctx, id)
		if err != nil {
			p.logger.Warn("failed to fetch hn item", "item_id", id, "error", err)
			continue
		}
		if len(rawStory) == 0 || storyID <= 0 {
			continue
		}

		messageKey := strconv.FormatInt(storyID, 10)
		message := kafkago.Message{
			Key:   []byte(messageKey),
			Value: rawStory,
			Time:  time.Now().UTC(),
		}

		if err := p.writer.WriteMessages(ctx, message); err != nil {
			return fmt.Errorf("write kafka message: %w", err)
		}

		p.seen.Add(storyID)
		published++
		processed++
	}

	p.logger.Info("hn poll complete", "fetched_ids", len(ids), "published_items", published, "processed_items", processed)
	return nil
}

func (p *hnProducer) fetchNewStoryIDs(ctx context.Context) ([]int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, newStoriesURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create hn ids request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "global-curiosity-engine/hn-producer")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch new stories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("hn new stories returned status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read new stories response: %w", err)
	}

	var ids []int64
	if err := json.Unmarshal(body, &ids); err != nil {
		return nil, fmt.Errorf("decode new stories response: %w", err)
	}

	if len(ids) > maxIDsPerPoll {
		ids = ids[:maxIDsPerPoll]
	}

	return ids, nil
}

func (p *hnProducer) fetchStory(ctx context.Context, id int64) (json.RawMessage, int64, error) {
	url := fmt.Sprintf(itemURLTemplate, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("create hn item request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "global-curiosity-engine/hn-producer")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("fetch hn item: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, 0, fmt.Errorf("hn item returned status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if err != nil {
		return nil, 0, fmt.Errorf("read hn item response: %w", err)
	}

	payload := strings.TrimSpace(string(body))
	if payload == "" || payload == "null" {
		return nil, 0, nil
	}

	var story hnStory
	if err := json.Unmarshal(body, &story); err != nil {
		return nil, 0, fmt.Errorf("decode hn item: %w", err)
	}

	if story.ID <= 0 || story.Deleted || story.Dead || story.Type != "story" {
		return nil, 0, nil
	}

	return json.RawMessage(body), story.ID, nil
}

type recentHNIDSet struct {
	capacity int
	order    []int64
	items    map[int64]struct{}
}

func newRecentHNIDSet(capacity int) *recentHNIDSet {
	return &recentHNIDSet{
		capacity: capacity,
		order:    make([]int64, 0, capacity),
		items:    make(map[int64]struct{}, capacity),
	}
}

func (s *recentHNIDSet) Contains(id int64) bool {
	_, ok := s.items[id]
	return ok
}

func (s *recentHNIDSet) Add(id int64) {
	if id <= 0 {
		return
	}
	if _, exists := s.items[id]; exists {
		return
	}

	s.items[id] = struct{}{}
	s.order = append(s.order, id)

	if len(s.order) <= s.capacity {
		return
	}

	oldest := s.order[0]
	s.order = s.order[1:]
	delete(s.items, oldest)
}

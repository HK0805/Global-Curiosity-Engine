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
	"strings"
	"syscall"
	"time"

	"github.com/HK0805/Global-Curiosity-Engine/internal/config"
	internalkafka "github.com/HK0805/Global-Curiosity-Engine/internal/kafka"
	"github.com/HK0805/Global-Curiosity-Engine/internal/logging"
	kafkago "github.com/segmentio/kafka-go"
)

const (
	serviceName          = "reddit-producer"
	topicName            = "raw.reddit.posts"
	redditFeedURL        = "https://api.reddit.com/r/all/new?limit=25&raw_json=1"
	pollInterval         = 30 * time.Second
	maxResponseBodyBytes = 8 * 1024 * 1024
	recentIDCapacity     = 2000
	defaultUserAgent     = "global-curiosity-engine:com.hk0805.globalcuriosityengine:v1.0 (by /u/example)"
)

type redditListing struct {
	Data struct {
		Children []redditChild `json:"children"`
	} `json:"data"`
}

type redditChild struct {
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data"`
}

type redditPostEnvelope struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Title     string `json:"title"`
	Subreddit string `json:"subreddit"`
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
		"feed_url", redditFeedURL,
		"reddit_user_agent_set", cfg.RedditUserAgent != "",
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	producer := newRedditProducer(cfg, writer, logger)

	if err := producer.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("service stopped with error", "error", err)
	}

	logger.Info("service shutting down")
}

type redditProducer struct {
	client    *http.Client
	userAgent string
	writer    *kafkago.Writer
	logger    *slog.Logger
	seen      *recentIDSet
}

func newRedditProducer(cfg config.Config, writer *kafkago.Writer, logger *slog.Logger) *redditProducer {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}

	userAgent := strings.TrimSpace(cfg.RedditUserAgent)
	if userAgent == "" {
		userAgent = defaultUserAgent
	}

	return &redditProducer{
		client: &http.Client{
			Transport: transport,
			Timeout:   20 * time.Second,
		},
		userAgent: userAgent,
		writer:    writer,
		logger:    logger,
		seen:      newRecentIDSet(recentIDCapacity),
	}
}

func (p *redditProducer) Run(ctx context.Context) error {
	p.logger.Info("poller ready", "poll_interval", pollInterval.String())

	if err := p.pollOnce(ctx); err != nil {
		p.logger.Warn("initial reddit poll failed", "error", err)
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := p.pollOnce(ctx); err != nil {
				p.logger.Warn("reddit poll failed", "error", err)
			}
		}
	}
}

func (p *redditProducer) pollOnce(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, redditFeedURL, nil)
	if err != nil {
		return fmt.Errorf("create reddit request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", p.userAgent)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch reddit feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("reddit returned status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if err != nil {
		return fmt.Errorf("read reddit response: %w", err)
	}

	var listing redditListing
	if err := json.Unmarshal(body, &listing); err != nil {
		return fmt.Errorf("decode reddit listing: %w", err)
	}

	published := 0
	for _, child := range listing.Data.Children {
		if len(child.Data) == 0 {
			continue
		}

		postID, err := redditPostID(child.Data)
		if err != nil {
			p.logger.Warn("skipping malformed reddit post", "error", err)
			continue
		}
		if postID == "" {
			continue
		}
		if p.seen.Contains(postID) {
			continue
		}

		message := kafkago.Message{
			Key:   []byte(postID),
			Value: child.Data,
			Time:  time.Now().UTC(),
		}

		if err := p.writer.WriteMessages(ctx, message); err != nil {
			return fmt.Errorf("write kafka message: %w", err)
		}

		p.seen.Add(postID)
		published++
	}

	p.logger.Info("reddit poll complete", "fetched_items", len(listing.Data.Children), "published_items", published)
	return nil
}

func redditPostID(raw json.RawMessage) (string, error) {
	var post redditPostEnvelope
	if err := json.Unmarshal(raw, &post); err != nil {
		return "", fmt.Errorf("decode reddit post: %w", err)
	}

	switch {
	case strings.TrimSpace(post.Name) != "":
		return strings.TrimSpace(post.Name), nil
	case strings.TrimSpace(post.ID) != "":
		return strings.TrimSpace(post.ID), nil
	default:
		return "", nil
	}
}

type recentIDSet struct {
	capacity int
	order    []string
	items    map[string]struct{}
}

func newRecentIDSet(capacity int) *recentIDSet {
	return &recentIDSet{
		capacity: capacity,
		order:    make([]string, 0, capacity),
		items:    make(map[string]struct{}, capacity),
	}
}

func (s *recentIDSet) Contains(id string) bool {
	_, ok := s.items[id]
	return ok
}

func (s *recentIDSet) Add(id string) {
	if id == "" {
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

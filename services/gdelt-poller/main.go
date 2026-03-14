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
	serviceName          = "gdelt-poller"
	topicName            = "raw.gdelt.events"
	gdeltQueryURL        = "https://api.gdeltproject.org/api/v2/doc/doc?query=language:english&mode=artlist&maxrecords=25&timespan=15min&format=json"
	pollInterval         = 60 * time.Second
	maxResponseBodyBytes = 8 * 1024 * 1024
	recentIDCapacity     = 5000
	defaultUserAgent     = "global-curiosity-engine/gdelt-poller"
)

type gdeltResponse struct {
	Articles []json.RawMessage `json:"articles"`
}

type gdeltArticle struct {
	URL           string `json:"url"`
	URLMobile     string `json:"url_mobile"`
	Title         string `json:"title"`
	Domain        string `json:"domain"`
	SeenDate      string `json:"seendate"`
	Language      string `json:"language"`
	SourceCountry string `json:"sourcecountry"`
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
		"query_url", gdeltQueryURL,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	poller := newGDELTPoller(writer, logger)

	if err := poller.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("service stopped with error", "error", err)
	}

	logger.Info("service shutting down")
}

type gdeltPoller struct {
	client *http.Client
	writer *kafkago.Writer
	logger *slog.Logger
	seen   *recentGDELTIDSet
}

func newGDELTPoller(writer *kafkago.Writer, logger *slog.Logger) *gdeltPoller {
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
		ResponseHeaderTimeout: 20 * time.Second,
	}

	return &gdeltPoller{
		client: &http.Client{
			Transport: transport,
			Timeout:   25 * time.Second,
		},
		writer: writer,
		logger: logger,
		seen:   newRecentGDELTIDSet(recentIDCapacity),
	}
}

func (p *gdeltPoller) Run(ctx context.Context) error {
	p.logger.Info("poller ready", "poll_interval", pollInterval.String())

	if err := p.pollOnce(ctx); err != nil {
		p.logger.Warn("initial gdelt poll failed", "error", err)
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := p.pollOnce(ctx); err != nil {
				p.logger.Warn("gdelt poll failed", "error", err)
			}
		}
	}
}

func (p *gdeltPoller) pollOnce(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gdeltQueryURL, nil)
	if err != nil {
		return fmt.Errorf("create gdelt request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", defaultUserAgent)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch gdelt feed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if err != nil {
		return fmt.Errorf("read gdelt response: %w", err)
	}

	trimmed := strings.TrimSpace(string(body))
	if throttleMessage, ok := gdeltThrottleMessage(resp.StatusCode, trimmed); ok {
		return fmt.Errorf("gdelt rate limited: %s", throttleMessage)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gdelt returned status %s: %s", resp.Status, trimmed)
	}

	if trimmed == "" {
		return nil
	}

	var payload gdeltResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("decode gdelt response: %w", err)
	}

	published := 0
	for _, rawArticle := range payload.Articles {
		if len(rawArticle) == 0 {
			continue
		}

		itemID, err := gdeltItemID(rawArticle)
		if err != nil {
			p.logger.Warn("skipping malformed gdelt item", "error", err)
			continue
		}
		if itemID == "" || p.seen.Contains(itemID) {
			continue
		}

		message := kafkago.Message{
			Key:   []byte(itemID),
			Value: rawArticle,
			Time:  time.Now().UTC(),
		}

		if err := p.writer.WriteMessages(ctx, message); err != nil {
			return fmt.Errorf("write kafka message: %w", err)
		}

		p.seen.Add(itemID)
		published++
	}

	p.logger.Info("gdelt poll complete", "fetched_items", len(payload.Articles), "published_items", published)
	return nil
}

func gdeltItemID(raw json.RawMessage) (string, error) {
	var article gdeltArticle
	if err := json.Unmarshal(raw, &article); err != nil {
		return "", fmt.Errorf("decode gdelt article: %w", err)
	}

	switch {
	case strings.TrimSpace(article.URL) != "":
		return strings.TrimSpace(article.URL), nil
	case strings.TrimSpace(article.URLMobile) != "":
		return strings.TrimSpace(article.URLMobile), nil
	case strings.TrimSpace(article.Domain) != "" && strings.TrimSpace(article.Title) != "":
		return strings.TrimSpace(article.Domain) + "|" + strings.TrimSpace(article.Title), nil
	case strings.TrimSpace(article.SeenDate) != "" && strings.TrimSpace(article.Title) != "":
		return strings.TrimSpace(article.SeenDate) + "|" + strings.TrimSpace(article.Title), nil
	default:
		return "", nil
	}
}

func gdeltThrottleMessage(statusCode int, body string) (string, bool) {
	if body == "" {
		return "", false
	}

	lowerBody := strings.ToLower(body)
	if statusCode == http.StatusTooManyRequests ||
		strings.Contains(lowerBody, "please limit requests") ||
		strings.Contains(lowerBody, "too many requests") ||
		strings.Contains(lowerBody, "rate limit") {
		return body, true
	}

	return "", false
}

type recentGDELTIDSet struct {
	capacity int
	order    []string
	items    map[string]struct{}
}

func newRecentGDELTIDSet(capacity int) *recentGDELTIDSet {
	return &recentGDELTIDSet{
		capacity: capacity,
		order:    make([]string, 0, capacity),
		items:    make(map[string]struct{}, capacity),
	}
}

func (s *recentGDELTIDSet) Contains(id string) bool {
	_, ok := s.items[id]
	return ok
}

func (s *recentGDELTIDSet) Add(id string) {
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

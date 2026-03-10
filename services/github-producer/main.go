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
	serviceName          = "github-producer"
	topicName            = "raw.github.events"
	githubEventsURL      = "https://api.github.com/events"
	pollInterval         = 30 * time.Second
	maxResponseBodyBytes = 8 * 1024 * 1024
	recentIDCapacity     = 5000
	defaultUserAgent     = "global-curiosity-engine/github-producer"
)

type githubEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Repo struct {
		Name string `json:"name"`
	} `json:"repo"`
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
		"events_url", githubEventsURL,
		"github_token_set", cfg.GitHubToken != "",
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	producer := newGitHubProducer(cfg, writer, logger)

	if err := producer.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("service stopped with error", "error", err)
	}

	logger.Info("service shutting down")
}

type githubProducer struct {
	client    *http.Client
	token     string
	userAgent string
	writer    *kafkago.Writer
	logger    *slog.Logger
	seen      *recentGitHubIDSet
}

func newGitHubProducer(cfg config.Config, writer *kafkago.Writer, logger *slog.Logger) *githubProducer {
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

	return &githubProducer{
		client: &http.Client{
			Transport: transport,
			Timeout:   20 * time.Second,
		},
		token:     strings.TrimSpace(cfg.GitHubToken),
		userAgent: defaultUserAgent,
		writer:    writer,
		logger:    logger,
		seen:      newRecentGitHubIDSet(recentIDCapacity),
	}
}

func (p *githubProducer) Run(ctx context.Context) error {
	p.logger.Info("poller ready", "poll_interval", pollInterval.String())

	if waitFor, err := p.pollOnce(ctx); err != nil {
		p.logger.Warn("initial github poll failed", "error", err)
	} else if waitFor > 0 {
		p.logger.Warn("github rate limited on startup", "retry_in", waitFor.String())
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			waitFor, err := p.pollOnce(ctx)
			if err != nil {
				p.logger.Warn("github poll failed", "error", err)
				continue
			}
			if waitFor <= 0 {
				continue
			}

			p.logger.Warn("github rate limited", "retry_in", waitFor.String())

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(waitFor):
			}
		}
	}
}

func (p *githubProducer) pollOnce(ctx context.Context) (time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubEventsURL, nil)
	if err != nil {
		return 0, fmt.Errorf("create github request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", p.userAgent)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fetch github events: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		waitFor := rateLimitWait(resp.Header, time.Minute)
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		p.logger.Warn("github rate limit response", "status", resp.Status, "body", strings.TrimSpace(string(body)))
		return waitFor, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return 0, fmt.Errorf("github returned status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	if err != nil {
		return 0, fmt.Errorf("read github response: %w", err)
	}

	var events []json.RawMessage
	if err := json.Unmarshal(body, &events); err != nil {
		return 0, fmt.Errorf("decode github events: %w", err)
	}

	published := 0
	for _, rawEvent := range events {
		if len(rawEvent) == 0 {
			continue
		}

		eventID, err := githubEventID(rawEvent)
		if err != nil {
			p.logger.Warn("skipping malformed github event", "error", err)
			continue
		}
		if eventID == "" || p.seen.Contains(eventID) {
			continue
		}

		message := kafkago.Message{
			Key:   []byte(eventID),
			Value: rawEvent,
			Time:  time.Now().UTC(),
		}

		if err := p.writer.WriteMessages(ctx, message); err != nil {
			return 0, fmt.Errorf("write kafka message: %w", err)
		}

		p.seen.Add(eventID)
		published++
	}

	p.logger.Info("github poll complete", "fetched_items", len(events), "published_items", published)
	return 0, nil
}

func githubEventID(raw json.RawMessage) (string, error) {
	var event githubEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return "", fmt.Errorf("decode github event: %w", err)
	}

	return strings.TrimSpace(event.ID), nil
}

func rateLimitWait(headers http.Header, fallback time.Duration) time.Duration {
	reset := strings.TrimSpace(headers.Get("X-RateLimit-Reset"))
	if reset == "" {
		if retryAfter := retryAfterWait(headers.Get("Retry-After")); retryAfter > 0 {
			return retryAfter
		}
		return fallback
	}

	epochSeconds, err := strconv.ParseInt(reset, 10, 64)
	if err != nil {
		if retryAfter := retryAfterWait(headers.Get("Retry-After")); retryAfter > 0 {
			return retryAfter
		}
		return fallback
	}

	waitFor := time.Until(time.Unix(epochSeconds, 0))
	if waitFor <= 0 {
		return 5 * time.Second
	}

	return waitFor
}

func retryAfterWait(value string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || seconds <= 0 {
		return 0
	}

	return time.Duration(seconds) * time.Second
}

type recentGitHubIDSet struct {
	capacity int
	order    []string
	items    map[string]struct{}
}

func newRecentGitHubIDSet(capacity int) *recentGitHubIDSet {
	return &recentGitHubIDSet{
		capacity: capacity,
		order:    make([]string, 0, capacity),
		items:    make(map[string]struct{}, capacity),
	}
}

func (s *recentGitHubIDSet) Contains(id string) bool {
	_, ok := s.items[id]
	return ok
}

func (s *recentGitHubIDSet) Add(id string) {
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

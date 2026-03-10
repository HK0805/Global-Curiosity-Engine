package main

import (
	"bufio"
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
	serviceName              = "wiki-producer"
	topicName                = "raw.wikipedia.edits"
	wikimediaRecentChangeURL = "https://stream.wikimedia.org/v2/stream/recentchange"
	maxEventSizeBytes        = 1024 * 1024
)

type wikiEventEnvelope struct {
	ID    any    `json:"id"`
	Title string `json:"title"`
	Meta  struct {
		ID    string `json:"id"`
		Topic string `json:"topic"`
	} `json:"meta"`
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
		"stream_url", wikimediaRecentChangeURL,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	producer := newWikipediaProducer(writer, logger)

	if err := producer.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("service stopped with error", "error", err)
	}

	logger.Info("service shutting down")
}

type wikipediaProducer struct {
	client *http.Client
	writer *kafkago.Writer
	logger *slog.Logger
}

func newWikipediaProducer(writer *kafkago.Writer, logger *slog.Logger) *wikipediaProducer {
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

	return &wikipediaProducer{
		client: &http.Client{
			Transport: transport,
		},
		writer: writer,
		logger: logger,
	}
}

func (p *wikipediaProducer) Run(ctx context.Context) error {
	backoff := time.Second

	for {
		err := p.consumeStream(ctx)
		if err == nil || errors.Is(err, context.Canceled) {
			return err
		}

		p.logger.Warn("stream disconnected, retrying", "error", err, "retry_in", backoff.String())

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

func (p *wikipediaProducer) consumeStream(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wikimediaRecentChangeURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", "global-curiosity-engine/wiki-producer")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("connect to wikimedia stream: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status from wikimedia stream: %s", resp.Status)
	}

	p.logger.Info("connected to wikimedia recent-change stream")

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxEventSizeBytes)

	var dataLines []string

	for scanner.Scan() {
		line := scanner.Text()

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if line == "" {
			if err := p.handleEvent(ctx, dataLines); err != nil {
				if !errors.Is(err, context.Canceled) {
					p.logger.Warn("failed to publish wikipedia event", "error", err)
				}
			}
			dataLines = dataLines[:0]
			continue
		}

		if strings.HasPrefix(line, ":") {
			continue
		}

		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}

	if err := scanner.Err(); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("read wikimedia stream: %w", err)
	}

	if len(dataLines) > 0 {
		if err := p.handleEvent(ctx, dataLines); err != nil {
			if !errors.Is(err, context.Canceled) {
				p.logger.Warn("failed to publish wikipedia event", "error", err)
			}
		}
	}

	return errors.New("wikimedia stream closed")
}

func (p *wikipediaProducer) handleEvent(ctx context.Context, dataLines []string) error {
	if len(dataLines) == 0 {
		return nil
	}

	payload := strings.TrimSpace(strings.Join(dataLines, "\n"))
	if payload == "" {
		return nil
	}

	key, err := extractMessageKey(payload)
	if err != nil {
		p.logger.Warn("skipping malformed wikipedia payload", "error", err)
		return nil
	}

	message := kafkago.Message{
		Key:   []byte(key),
		Value: []byte(payload),
		Time:  time.Now().UTC(),
	}

	if err := p.writer.WriteMessages(ctx, message); err != nil {
		return fmt.Errorf("write kafka message: %w", err)
	}

	return nil
}

func extractMessageKey(payload string) (string, error) {
	var event wikiEventEnvelope
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return "", fmt.Errorf("decode payload: %w", err)
	}

	switch {
	case strings.TrimSpace(event.Meta.ID) != "":
		return strings.TrimSpace(event.Meta.ID), nil
	case strings.TrimSpace(event.Title) != "":
		return strings.TrimSpace(event.Title), nil
	case event.ID != nil:
		return fmt.Sprint(event.ID), nil
	default:
		return "", nil
	}
}

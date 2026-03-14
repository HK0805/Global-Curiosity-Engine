package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/HK0805/Global-Curiosity-Engine/internal/config"
	internalkafka "github.com/HK0805/Global-Curiosity-Engine/internal/kafka"
	"github.com/HK0805/Global-Curiosity-Engine/internal/logging"
	"github.com/HK0805/Global-Curiosity-Engine/internal/models"
	kafkago "github.com/segmentio/kafka-go"
)

const (
	serviceName = "topic-extractor"
	inputTopic  = "normalized.events"
	outputTopic = "topics.extracted"
	groupID     = "topic-extractor"
	minTokenLen = 3
)

var stopWords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {}, "by": {},
	"for": {}, "from": {}, "has": {}, "in": {}, "into": {}, "is": {}, "it": {}, "its": {},
	"of": {}, "on": {}, "or": {}, "that": {}, "the": {}, "their": {}, "this": {}, "to": {},
	"was": {}, "were": {}, "will": {}, "with": {}, "after": {}, "amid": {}, "over": {},
	"under": {}, "about": {}, "than": {}, "new": {}, "more": {}, "just": {}, "via": {},
}

func main() {
	cfg := config.Load()
	logger := logging.New(serviceName)
	reader := internalkafka.NewReader(cfg, inputTopic, groupID)
	defer func() {
		if err := reader.Close(); err != nil {
			logger.Error("failed to close kafka reader", "error", err)
		}
	}()
	writer := internalkafka.NewWriter(cfg, outputTopic)
	defer func() {
		if err := writer.Close(); err != nil {
			logger.Error("failed to close kafka writer", "error", err)
		}
	}()

	logger.Info(
		"service starting",
		"kafka_broker", cfg.KafkaBroker,
		"input_topic", inputTopic,
		"output_topic", outputTopic,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	processor := newTopicExtractor(reader, writer, logger)

	if err := processor.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("service stopped with error", "error", err)
	}

	logger.Info("service shutting down")
}

type topicExtractor struct {
	reader *kafkago.Reader
	writer *kafkago.Writer
	logger *slog.Logger
}

func newTopicExtractor(reader *kafkago.Reader, writer *kafkago.Writer, logger *slog.Logger) *topicExtractor {
	return &topicExtractor{
		reader: reader,
		writer: writer,
		logger: logger,
	}
}

func (t *topicExtractor) Run(ctx context.Context) error {
	t.logger.Info("topic extractor ready")

	for {
		message, err := t.reader.ReadMessage(ctx)
		if err != nil {
			return err
		}

		var event models.NormalizedEvent
		if err := json.Unmarshal(message.Value, &event); err != nil {
			t.logger.Warn("failed to decode normalized event", "error", err)
			continue
		}

		if strings.TrimSpace(event.ID) == "" || strings.TrimSpace(event.Source) == "" || strings.TrimSpace(event.Title) == "" {
			t.logger.Warn("skipping incomplete normalized event", "event_id", event.ID, "source", event.Source)
			continue
		}

		topics := extractTopics(event.Title)
		if len(topics) == 0 {
			continue
		}

		for _, candidate := range topics {
			extracted := models.ExtractedTopicEvent{
				ID:            event.ID + ":" + candidate.Topic,
				SourceEventID: event.ID,
				Source:        event.Source,
				Topic:         candidate.Topic,
				TopicType:     "token",
				MentionCount:  1,
				Timestamp:     event.Timestamp,
				Metadata: map[string]any{
					"original_title": event.Title,
					"token_position": candidate.Position,
				},
			}

			payload, err := json.Marshal(extracted)
			if err != nil {
				t.logger.Warn("failed to encode extracted topic", "event_id", event.ID, "topic", candidate.Topic, "error", err)
				continue
			}

			if err := t.writer.WriteMessages(ctx, kafkago.Message{
				Key:   []byte(extracted.ID),
				Value: payload,
				Time:  time.Now().UTC(),
			}); err != nil {
				return fmt.Errorf("write extracted topic message: %w", err)
			}
		}

		t.logger.Info("extracted topics published", "event_id", event.ID, "source", event.Source, "topic_count", len(topics))
	}
}

type topicCandidate struct {
	Topic    string
	Position int
}

func extractTopics(title string) []topicCandidate {
	tokens := strings.FieldsFunc(strings.ToLower(title), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})

	seen := make(map[string]struct{}, len(tokens))
	candidates := make([]topicCandidate, 0, len(tokens))

	for idx, token := range tokens {
		token = strings.TrimSpace(token)
		if !isMeaningfulToken(token) {
			continue
		}
		if _, exists := seen[token]; exists {
			continue
		}

		seen[token] = struct{}{}
		candidates = append(candidates, topicCandidate{
			Topic:    token,
			Position: idx,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Position < candidates[j].Position
	})

	return candidates
}

func isMeaningfulToken(token string) bool {
	if len(token) < minTokenLen {
		return false
	}
	if _, blocked := stopWords[token]; blocked {
		return false
	}

	hasLetter := false
	for _, r := range token {
		if unicode.IsLetter(r) {
			hasLetter = true
			break
		}
	}

	return hasLetter
}

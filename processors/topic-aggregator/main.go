package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/signal"
	"strings"
	"syscall"
	"time"

	appdb "github.com/HK0805/Global-Curiosity-Engine/database"
	"github.com/HK0805/Global-Curiosity-Engine/internal/config"
	internalkafka "github.com/HK0805/Global-Curiosity-Engine/internal/kafka"
	"github.com/HK0805/Global-Curiosity-Engine/internal/logging"
	"github.com/HK0805/Global-Curiosity-Engine/internal/models"
	kafkago "github.com/segmentio/kafka-go"
)

const (
	serviceName = "topic-aggregator"
	inputTopic  = "topics.extracted"
	outputTopic = "topics.metrics"
	groupID     = "topic-aggregator"
)

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
	store, err := appdb.Open(cfg.SQLitePath)
	if err != nil {
		logger.Error("failed to open sqlite store", "error", err, "sqlite_path", cfg.SQLitePath)
		return
	}
	defer func() {
		if err := store.Close(); err != nil {
			logger.Error("failed to close sqlite store", "error", err)
		}
	}()

	logger.Info(
		"service starting",
		"kafka_broker", cfg.KafkaBroker,
		"sqlite_path", cfg.SQLitePath,
		"input_topic", inputTopic,
		"output_topic", outputTopic,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	processor := newTopicAggregator(reader, writer, store, logger)

	if err := processor.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("service stopped with error", "error", err)
	}

	logger.Info("service shutting down")
}

type topicAggregator struct {
	reader *kafkago.Reader
	writer *kafkago.Writer
	store  *appdb.Store
	logger *slog.Logger
	counts map[string]*aggregateState
}

type aggregateState struct {
	Topic              string
	Source             string
	BucketStart        time.Time
	MentionCount       int
	LastEventTimestamp time.Time
	TopicType          string
}

func newTopicAggregator(reader *kafkago.Reader, writer *kafkago.Writer, store *appdb.Store, logger *slog.Logger) *topicAggregator {
	return &topicAggregator{
		reader: reader,
		writer: writer,
		store:  store,
		logger: logger,
		counts: make(map[string]*aggregateState),
	}
}

func (a *topicAggregator) Run(ctx context.Context) error {
	a.logger.Info("topic aggregator ready")

	for {
		message, err := a.reader.ReadMessage(ctx)
		if err != nil {
			return err
		}

		var event models.ExtractedTopicEvent
		if err := json.Unmarshal(message.Value, &event); err != nil {
			a.logger.Warn("failed to decode extracted topic event", "error", err)
			continue
		}

		if strings.TrimSpace(event.ID) == "" || strings.TrimSpace(event.Source) == "" || strings.TrimSpace(event.Topic) == "" {
			a.logger.Warn("skipping incomplete extracted topic event", "event_id", event.ID, "source", event.Source, "topic", event.Topic)
			continue
		}
		if event.Timestamp.IsZero() {
			a.logger.Warn("skipping extracted topic event with invalid timestamp", "event_id", event.ID, "topic", event.Topic)
			continue
		}

		metric := a.updateMetric(event)
		payload, err := json.Marshal(metric)
		if err != nil {
			a.logger.Warn("failed to encode topic metric", "metric_id", metric.ID, "error", err)
			continue
		}

		if err := a.writer.WriteMessages(ctx, kafkago.Message{
			Key:   []byte(metric.ID),
			Value: payload,
			Time:  time.Now().UTC(),
		}); err != nil {
			return fmt.Errorf("write topic metric message: %w", err)
		}
		if err := a.store.UpsertTopicMetric(ctx, metric); err != nil {
			a.logger.Warn("failed to persist topic metric", "metric_id", metric.ID, "error", err)
			continue
		}

		a.logger.Info(
			"topic metric published",
			"metric_id", metric.ID,
			"topic", metric.Topic,
			"source", metric.Source,
			"bucket_start", metric.BucketStart.Format(time.RFC3339),
			"mention_count", metric.MentionCount,
		)
	}
}

func (a *topicAggregator) updateMetric(event models.ExtractedTopicEvent) models.TopicMetric {
	bucketStart := event.Timestamp.UTC().Truncate(time.Minute)
	key := buildMetricID(event.Source, event.Topic, bucketStart)

	state, exists := a.counts[key]
	if !exists {
		state = &aggregateState{
			Topic:              event.Topic,
			Source:             event.Source,
			BucketStart:        bucketStart,
			LastEventTimestamp: event.Timestamp.UTC(),
			TopicType:          event.TopicType,
		}
		a.counts[key] = state
	}

	state.MentionCount++
	if event.Timestamp.UTC().After(state.LastEventTimestamp) {
		state.LastEventTimestamp = event.Timestamp.UTC()
	}
	if strings.TrimSpace(event.TopicType) != "" {
		state.TopicType = event.TopicType
	}

	return models.TopicMetric{
		ID:                 key,
		Topic:              state.Topic,
		Source:             state.Source,
		BucketStart:        state.BucketStart,
		MentionCount:       state.MentionCount,
		LastEventTimestamp: state.LastEventTimestamp,
		Metadata: map[string]any{
			"aggregation": "minute",
			"bucket_size": "1m",
			"topic_type":  state.TopicType,
		},
	}
}

func buildMetricID(source, topic string, bucketStart time.Time) string {
	return source + ":" + topic + ":" + bucketStart.UTC().Format(time.RFC3339)
}

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os/signal"
	"sort"
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
	serviceName = "curiosity-scorer"
	inputTopic  = "topics.metrics"
	outputTopic = "topics.scored"
	groupID     = "curiosity-scorer"
)

var sourceWeights = map[string]float64{
	"wikipedia":  1.3,
	"reddit":     1.0,
	"hackernews": 1.1,
	"github":     0.9,
	"gdelt":      1.2,
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

	processor := newCuriosityScorer(reader, writer, store, logger)

	if err := processor.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("service stopped with error", "error", err)
	}

	logger.Info("service shutting down")
}

type curiosityScorer struct {
	reader  *kafkago.Reader
	writer  *kafkago.Writer
	store   *appdb.Store
	logger  *slog.Logger
	metrics map[string]models.TopicMetric
}

func newCuriosityScorer(reader *kafkago.Reader, writer *kafkago.Writer, store *appdb.Store, logger *slog.Logger) *curiosityScorer {
	return &curiosityScorer{
		reader:  reader,
		writer:  writer,
		store:   store,
		logger:  logger,
		metrics: make(map[string]models.TopicMetric),
	}
}

func (c *curiosityScorer) Run(ctx context.Context) error {
	c.logger.Info("curiosity scorer ready")

	for {
		message, err := c.reader.ReadMessage(ctx)
		if err != nil {
			return err
		}

		var metric models.TopicMetric
		if err := json.Unmarshal(message.Value, &metric); err != nil {
			c.logger.Warn("failed to decode topic metric", "error", err)
			continue
		}

		if strings.TrimSpace(metric.ID) == "" || strings.TrimSpace(metric.Topic) == "" || strings.TrimSpace(metric.Source) == "" {
			c.logger.Warn("skipping incomplete topic metric", "metric_id", metric.ID, "topic", metric.Topic, "source", metric.Source)
			continue
		}
		if metric.BucketStart.IsZero() || metric.LastEventTimestamp.IsZero() {
			c.logger.Warn("skipping topic metric with invalid timestamps", "metric_id", metric.ID, "topic", metric.Topic)
			continue
		}
		if metric.MentionCount < 0 {
			c.logger.Warn("skipping topic metric with invalid mention count", "metric_id", metric.ID, "mention_count", metric.MentionCount)
			continue
		}

		c.metrics[metric.ID] = metric
		score := c.computeScore(metric.Topic)

		payload, err := json.Marshal(score)
		if err != nil {
			c.logger.Warn("failed to encode topic score", "topic", score.Topic, "error", err)
			continue
		}

		if err := c.writer.WriteMessages(ctx, kafkago.Message{
			Key:   []byte(score.Topic),
			Value: payload,
			Time:  time.Now().UTC(),
		}); err != nil {
			return fmt.Errorf("write topic score message: %w", err)
		}
		if err := c.store.UpsertTopicScore(ctx, score); err != nil {
			c.logger.Warn("failed to persist topic score", "topic", score.Topic, "score_id", score.ID, "error", err)
			continue
		}

		c.logger.Info(
			"topic score published",
			"topic", score.Topic,
			"score", score.Score,
			"distinct_sources", score.DistinctSources,
			"total_mentions", score.TotalMentions,
		)
	}
}

func (c *curiosityScorer) computeScore(topic string) models.TopicScore {
	type sourceSummary struct {
		Source   string
		Mentions int
		Weight   float64
	}

	relevant := make([]models.TopicMetric, 0)
	for _, metric := range c.metrics {
		if metric.Topic == topic {
			relevant = append(relevant, metric)
		}
	}

	sourceSet := make(map[string]struct{})
	sourceMentions := make(map[string]int)
	weightedSum := 0.0
	totalMentions := 0
	latestTimestamp := time.Time{}

	for _, metric := range relevant {
		weight := sourceWeight(metric.Source)
		weightedSum += float64(metric.MentionCount) * weight
		totalMentions += metric.MentionCount
		sourceSet[metric.Source] = struct{}{}
		sourceMentions[metric.Source] += metric.MentionCount
		if metric.LastEventTimestamp.After(latestTimestamp) {
			latestTimestamp = metric.LastEventTimestamp
		}
		if metric.BucketStart.After(latestTimestamp) {
			latestTimestamp = metric.BucketStart
		}
	}

	distinctSources := len(sourceSet)
	scoreValue := weightedSum
	if distinctSources > 1 {
		scoreValue = weightedSum * (1 + 0.15*float64(distinctSources-1))
	}

	breakdown := make([]sourceSummary, 0, len(sourceMentions))
	for source, mentions := range sourceMentions {
		breakdown = append(breakdown, sourceSummary{
			Source:   source,
			Mentions: mentions,
			Weight:   sourceWeight(source),
		})
	}
	sort.Slice(breakdown, func(i, j int) bool {
		return breakdown[i].Source < breakdown[j].Source
	})

	sourceBreakdown := make([]map[string]any, 0, len(breakdown))
	for _, item := range breakdown {
		sourceBreakdown = append(sourceBreakdown, map[string]any{
			"source":   item.Source,
			"mentions": item.Mentions,
			"weight":   item.Weight,
		})
	}

	return models.TopicScore{
		ID:              buildScoreID(topic, latestTimestamp),
		Topic:           topic,
		Score:           roundScore(scoreValue),
		DistinctSources: distinctSources,
		TotalMentions:   totalMentions,
		Timestamp:       latestTimestamp.UTC(),
		Metadata: map[string]any{
			"formula_version":  "v1",
			"weighted_sum":     roundScore(weightedSum),
			"source_breakdown": sourceBreakdown,
		},
	}
}

func sourceWeight(source string) float64 {
	if weight, ok := sourceWeights[source]; ok {
		return weight
	}
	return 1.0
}

func buildScoreID(topic string, ts time.Time) string {
	return topic + ":" + ts.UTC().Format(time.RFC3339)
}

func roundScore(value float64) float64 {
	return math.Round(value*1000) / 1000
}

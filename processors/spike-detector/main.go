package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
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
	serviceName = "spike-detector"
	inputTopic  = "topics.scored"
	outputTopic = "topics.spikes"
	groupID     = "spike-detector"

	cooldownWindow          = 5 * time.Minute
	initialSurgeThreshold   = 8.0
	scoreJumpRatioThreshold = 1.5
	scoreJumpMinIncrease    = 3.0
	cooldownBypassIncrease  = 6.0
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

	processor := newSpikeDetector(reader, writer, store, logger)

	if err := processor.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("service stopped with error", "error", err)
	}

	logger.Info("service shutting down")
}

type spikeDetector struct {
	reader *kafkago.Reader
	writer *kafkago.Writer
	store  *appdb.Store
	logger *slog.Logger
	state  map[string]topicState
}

type topicState struct {
	LastScore     float64
	LastTimestamp time.Time
	LastSpikeAt   time.Time
}

func newSpikeDetector(reader *kafkago.Reader, writer *kafkago.Writer, store *appdb.Store, logger *slog.Logger) *spikeDetector {
	return &spikeDetector{
		reader: reader,
		writer: writer,
		store:  store,
		logger: logger,
		state:  make(map[string]topicState),
	}
}

func (d *spikeDetector) Run(ctx context.Context) error {
	d.logger.Info("spike detector ready")

	for {
		message, err := d.reader.ReadMessage(ctx)
		if err != nil {
			return err
		}

		var score models.TopicScore
		if err := json.Unmarshal(message.Value, &score); err != nil {
			d.logger.Warn("failed to decode topic score", "error", err)
			continue
		}

		if strings.TrimSpace(score.Topic) == "" || score.Timestamp.IsZero() {
			d.logger.Warn("skipping invalid topic score", "topic", score.Topic, "timestamp", score.Timestamp)
			continue
		}
		if score.Score < 0 {
			d.logger.Warn("skipping topic score with invalid score value", "topic", score.Topic, "score", score.Score)
			continue
		}

		spike, emit := d.evaluate(score)
		if !emit {
			continue
		}

		payload, err := json.Marshal(spike)
		if err != nil {
			d.logger.Warn("failed to encode topic spike", "topic", spike.Topic, "error", err)
			continue
		}

		if err := d.writer.WriteMessages(ctx, kafkago.Message{
			Key:   []byte(spike.Topic),
			Value: payload,
			Time:  time.Now().UTC(),
		}); err != nil {
			return fmt.Errorf("write topic spike message: %w", err)
		}
		if err := d.store.UpsertTopicSpike(ctx, spike); err != nil {
			d.logger.Warn("failed to persist topic spike", "topic", spike.Topic, "spike_id", spike.ID, "error", err)
			continue
		}

		d.logger.Info(
			"topic spike published",
			"topic", spike.Topic,
			"spike_type", spike.SpikeType,
			"previous_score", spike.PreviousScore,
			"current_score", spike.CurrentScore,
		)
	}
}

func (d *spikeDetector) evaluate(score models.TopicScore) (models.TopicSpike, bool) {
	currentTime := score.Timestamp.UTC()
	state := d.state[score.Topic]
	previousScore := state.LastScore

	increaseAbsolute := score.Score - previousScore
	increaseRatio := 0.0
	if previousScore > 0 {
		increaseRatio = score.Score / previousScore
	}

	spikeType := ""
	if previousScore <= 0 {
		if score.Score >= initialSurgeThreshold {
			spikeType = "initial_surge"
		}
	} else if score.Score >= previousScore*scoreJumpRatioThreshold && increaseAbsolute >= scoreJumpMinIncrease {
		if state.LastSpikeAt.IsZero() || currentTime.Sub(state.LastSpikeAt) >= cooldownWindow || increaseAbsolute >= cooldownBypassIncrease {
			spikeType = "score_jump"
		}
	}

	d.state[score.Topic] = topicState{
		LastScore:     score.Score,
		LastTimestamp: currentTime,
		LastSpikeAt:   chooseLastSpikeAt(state.LastSpikeAt, currentTime, spikeType != ""),
	}

	if spikeType == "" {
		return models.TopicSpike{}, false
	}

	return models.TopicSpike{
		ID:               buildSpikeID(score.Topic, currentTime),
		Topic:            score.Topic,
		SpikeType:        spikeType,
		PreviousScore:    roundValue(previousScore),
		CurrentScore:     roundValue(score.Score),
		IncreaseRatio:    roundValue(increaseRatio),
		IncreaseAbsolute: roundValue(increaseAbsolute),
		DistinctSources:  score.DistinctSources,
		TotalMentions:    score.TotalMentions,
		Timestamp:        currentTime,
		Metadata: map[string]any{
			"formula_version":       "v1",
			"cooldown_seconds":      int(cooldownWindow.Seconds()),
			"cooldown_bypass_delta": cooldownBypassIncrease,
			"score_metadata":        score.Metadata,
		},
	}, true
}

func chooseLastSpikeAt(previous time.Time, current time.Time, emitted bool) time.Time {
	if emitted {
		return current
	}
	return previous
}

func buildSpikeID(topic string, ts time.Time) string {
	return topic + ":" + ts.UTC().Format(time.RFC3339)
}

func roundValue(value float64) float64 {
	return math.Round(value*1000) / 1000
}

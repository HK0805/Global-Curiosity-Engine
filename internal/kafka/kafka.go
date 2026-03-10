package kafka

import (
	"strings"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/HK0805/Global-Curiosity-Engine/internal/config"
)

func Brokers(cfg config.Config) []string {
	parts := strings.Split(cfg.KafkaBroker, ",")
	brokers := make([]string, 0, len(parts))

	for _, part := range parts {
		broker := strings.TrimSpace(part)
		if broker != "" {
			brokers = append(brokers, broker)
		}
	}

	if len(brokers) == 0 {
		return []string{"kafka:29092"}
	}

	return brokers
}

func NewWriter(cfg config.Config, topic string) *kafkago.Writer {
	return &kafkago.Writer{
		Addr:         kafkago.TCP(Brokers(cfg)...),
		Topic:        topic,
		RequiredAcks: kafkago.RequireOne,
		Async:        false,
		Balancer:     &kafkago.LeastBytes{},
	}
}

func NewReader(cfg config.Config, topic, groupID string) *kafkago.Reader {
	return kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        Brokers(cfg),
		GroupID:        groupID,
		Topic:          topic,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: time.Second,
		StartOffset:    kafkago.FirstOffset,
	})
}

func NewGroupReader(cfg config.Config, topics []string, groupID string) *kafkago.Reader {
	return kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:        Brokers(cfg),
		GroupID:        groupID,
		GroupTopics:    topics,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: time.Second,
		StartOffset:    kafkago.FirstOffset,
	})
}

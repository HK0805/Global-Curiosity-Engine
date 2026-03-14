package models

import "time"

type RawEvent struct {
	ID        string         `json:"id"`
	Source    string         `json:"source"`
	Title     string         `json:"title"`
	URL       string         `json:"url"`
	Timestamp time.Time      `json:"timestamp"`
	Metadata  map[string]any `json:"metadata"`
	Payload   map[string]any `json:"payload"`
}

type NormalizedEvent struct {
	ID        string         `json:"id"`
	Source    string         `json:"source"`
	Title     string         `json:"title"`
	URL       string         `json:"url"`
	Timestamp time.Time      `json:"timestamp"`
	Metadata  map[string]any `json:"metadata"`
}

type ExtractedTopicEvent struct {
	ID            string         `json:"id"`
	SourceEventID string         `json:"source_event_id"`
	Source        string         `json:"source"`
	Topic         string         `json:"topic"`
	TopicType     string         `json:"topic_type"`
	MentionCount  int            `json:"mention_count"`
	Timestamp     time.Time      `json:"timestamp"`
	Metadata      map[string]any `json:"metadata"`
}

type TopicMetric struct {
	ID                 string         `json:"id"`
	Topic              string         `json:"topic"`
	Source             string         `json:"source"`
	BucketStart        time.Time      `json:"bucket_start"`
	MentionCount       int            `json:"mention_count"`
	LastEventTimestamp time.Time      `json:"last_event_timestamp"`
	Metadata           map[string]any `json:"metadata"`
}

type TopicScore struct {
	ID              string         `json:"id"`
	Topic           string         `json:"topic"`
	Score           float64        `json:"score"`
	DistinctSources int            `json:"distinct_sources"`
	TotalMentions   int            `json:"total_mentions"`
	Timestamp       time.Time      `json:"timestamp"`
	Metadata        map[string]any `json:"metadata"`
}

type TopicSpike struct {
	ID               string         `json:"id"`
	Topic            string         `json:"topic"`
	SpikeType        string         `json:"spike_type"`
	PreviousScore    float64        `json:"previous_score"`
	CurrentScore     float64        `json:"current_score"`
	IncreaseRatio    float64        `json:"increase_ratio"`
	IncreaseAbsolute float64        `json:"increase_absolute"`
	DistinctSources  int            `json:"distinct_sources"`
	TotalMentions    int            `json:"total_mentions"`
	Timestamp        time.Time      `json:"timestamp"`
	Metadata         map[string]any `json:"metadata"`
}

type SourceCount struct {
	Source string `json:"source"`
	Count  int64  `json:"count"`
}

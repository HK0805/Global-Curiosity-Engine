package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/HK0805/Global-Curiosity-Engine/internal/models"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS normalized_events (
	id TEXT PRIMARY KEY,
	source TEXT NOT NULL,
	title TEXT NOT NULL,
	url TEXT,
	timestamp TEXT NOT NULL,
	metadata_json TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_normalized_events_timestamp
	ON normalized_events(timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_normalized_events_source
	ON normalized_events(source);

CREATE TABLE IF NOT EXISTS topic_metrics (
	id TEXT PRIMARY KEY,
	topic TEXT NOT NULL,
	source TEXT NOT NULL,
	bucket_start TEXT NOT NULL,
	mention_count INTEGER NOT NULL,
	last_event_timestamp TEXT NOT NULL,
	metadata_json TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_topic_metrics_topic
	ON topic_metrics(topic);

CREATE INDEX IF NOT EXISTS idx_topic_metrics_bucket_start
	ON topic_metrics(bucket_start DESC);

CREATE TABLE IF NOT EXISTS topic_scores (
	id TEXT PRIMARY KEY,
	topic TEXT NOT NULL,
	score REAL NOT NULL,
	distinct_sources INTEGER NOT NULL,
	total_mentions INTEGER NOT NULL,
	timestamp TEXT NOT NULL,
	metadata_json TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_topic_scores_topic
	ON topic_scores(topic);

CREATE INDEX IF NOT EXISTS idx_topic_scores_timestamp
	ON topic_scores(timestamp DESC);

CREATE TABLE IF NOT EXISTS topic_spikes (
	id TEXT PRIMARY KEY,
	topic TEXT NOT NULL,
	spike_type TEXT NOT NULL,
	previous_score REAL NOT NULL,
	current_score REAL NOT NULL,
	increase_ratio REAL NOT NULL,
	increase_absolute REAL NOT NULL,
	distinct_sources INTEGER NOT NULL,
	total_mentions INTEGER NOT NULL,
	timestamp TEXT NOT NULL,
	metadata_json TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_topic_spikes_topic
	ON topic_spikes(topic);

CREATE INDEX IF NOT EXISTS idx_topic_spikes_timestamp
	ON topic_spikes(timestamp DESC);
`

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := ensureParentDir(path); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	db.SetMaxOpenConns(1)

	store := &Store{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) PingContext(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) UpsertNormalizedEvent(ctx context.Context, event models.NormalizedEvent) error {
	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO normalized_events (id, source, title, url, timestamp, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			source = excluded.source,
			title = excluded.title,
			url = excluded.url,
			timestamp = excluded.timestamp,
			metadata_json = excluded.metadata_json
	`,
		event.ID,
		event.Source,
		event.Title,
		event.URL,
		event.Timestamp.UTC().Format(timeLayout),
		string(metadataJSON),
	)
	if err != nil {
		return fmt.Errorf("upsert normalized event: %w", err)
	}

	return nil
}

func (s *Store) ListNormalizedEvents(ctx context.Context, limit int) ([]models.NormalizedEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, source, title, url, timestamp, metadata_json
		FROM normalized_events
		ORDER BY timestamp DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query normalized events: %w", err)
	}
	defer rows.Close()

	events := make([]models.NormalizedEvent, 0, limit)
	for rows.Next() {
		var (
			event        models.NormalizedEvent
			timestampRaw string
			metadataRaw  string
		)

		if err := rows.Scan(&event.ID, &event.Source, &event.Title, &event.URL, &timestampRaw, &metadataRaw); err != nil {
			return nil, fmt.Errorf("scan normalized event: %w", err)
		}

		if timestampRaw != "" {
			parsed, err := time.Parse(timeLayout, timestampRaw)
			if err != nil {
				return nil, fmt.Errorf("parse normalized event timestamp: %w", err)
			}
			event.Timestamp = parsed.UTC()
		}

		if metadataRaw != "" {
			if err := json.Unmarshal([]byte(metadataRaw), &event.Metadata); err != nil {
				return nil, fmt.Errorf("decode normalized event metadata: %w", err)
			}
		}
		if event.Metadata == nil {
			event.Metadata = map[string]any{}
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate normalized events: %w", err)
	}

	return events, nil
}

func (s *Store) UpsertTopicMetric(ctx context.Context, metric models.TopicMetric) error {
	metadataJSON, err := json.Marshal(metric.Metadata)
	if err != nil {
		return fmt.Errorf("marshal topic metric metadata: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO topic_metrics (id, topic, source, bucket_start, mention_count, last_event_timestamp, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			topic = excluded.topic,
			source = excluded.source,
			bucket_start = excluded.bucket_start,
			mention_count = excluded.mention_count,
			last_event_timestamp = excluded.last_event_timestamp,
			metadata_json = excluded.metadata_json
	`,
		metric.ID,
		metric.Topic,
		metric.Source,
		metric.BucketStart.UTC().Format(timeLayout),
		metric.MentionCount,
		metric.LastEventTimestamp.UTC().Format(timeLayout),
		string(metadataJSON),
	)
	if err != nil {
		return fmt.Errorf("upsert topic metric: %w", err)
	}

	return nil
}

func (s *Store) UpsertTopicScore(ctx context.Context, score models.TopicScore) error {
	metadataJSON, err := json.Marshal(score.Metadata)
	if err != nil {
		return fmt.Errorf("marshal topic score metadata: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO topic_scores (id, topic, score, distinct_sources, total_mentions, timestamp, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			topic = excluded.topic,
			score = excluded.score,
			distinct_sources = excluded.distinct_sources,
			total_mentions = excluded.total_mentions,
			timestamp = excluded.timestamp,
			metadata_json = excluded.metadata_json
	`,
		score.ID,
		score.Topic,
		score.Score,
		score.DistinctSources,
		score.TotalMentions,
		score.Timestamp.UTC().Format(timeLayout),
		string(metadataJSON),
	)
	if err != nil {
		return fmt.Errorf("upsert topic score: %w", err)
	}

	return nil
}

func (s *Store) UpsertTopicSpike(ctx context.Context, spike models.TopicSpike) error {
	metadataJSON, err := json.Marshal(spike.Metadata)
	if err != nil {
		return fmt.Errorf("marshal topic spike metadata: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO topic_spikes (id, topic, spike_type, previous_score, current_score, increase_ratio, increase_absolute, distinct_sources, total_mentions, timestamp, metadata_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			topic = excluded.topic,
			spike_type = excluded.spike_type,
			previous_score = excluded.previous_score,
			current_score = excluded.current_score,
			increase_ratio = excluded.increase_ratio,
			increase_absolute = excluded.increase_absolute,
			distinct_sources = excluded.distinct_sources,
			total_mentions = excluded.total_mentions,
			timestamp = excluded.timestamp,
			metadata_json = excluded.metadata_json
	`,
		spike.ID,
		spike.Topic,
		spike.SpikeType,
		spike.PreviousScore,
		spike.CurrentScore,
		spike.IncreaseRatio,
		spike.IncreaseAbsolute,
		spike.DistinctSources,
		spike.TotalMentions,
		spike.Timestamp.UTC().Format(timeLayout),
		string(metadataJSON),
	)
	if err != nil {
		return fmt.Errorf("upsert topic spike: %w", err)
	}

	return nil
}

func (s *Store) ListTrendingTopicScores(ctx context.Context, limit int) ([]models.TopicScore, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ts.id, ts.topic, ts.score, ts.distinct_sources, ts.total_mentions, ts.timestamp, ts.metadata_json
		FROM topic_scores ts
		INNER JOIN (
			SELECT topic, MAX(timestamp) AS max_timestamp
			FROM topic_scores
			GROUP BY topic
		) latest
			ON latest.topic = ts.topic AND latest.max_timestamp = ts.timestamp
		ORDER BY ts.score DESC, ts.timestamp DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query trending topic scores: %w", err)
	}
	defer rows.Close()

	scores := make([]models.TopicScore, 0, limit)
	for rows.Next() {
		item, err := scanTopicScore(rows)
		if err != nil {
			return nil, err
		}
		scores = append(scores, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trending topic scores: %w", err)
	}

	return scores, nil
}

func (s *Store) ListTopicSpikes(ctx context.Context, limit int) ([]models.TopicSpike, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, topic, spike_type, previous_score, current_score, increase_ratio, increase_absolute, distinct_sources, total_mentions, timestamp, metadata_json
		FROM topic_spikes
		ORDER BY timestamp DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query topic spikes: %w", err)
	}
	defer rows.Close()

	spikes := make([]models.TopicSpike, 0, limit)
	for rows.Next() {
		item, err := scanTopicSpike(rows)
		if err != nil {
			return nil, err
		}
		spikes = append(spikes, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate topic spikes: %w", err)
	}

	return spikes, nil
}

func (s *Store) CuriosityIndex(ctx context.Context, limit int) (float64, int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ts.id, ts.topic, ts.score, ts.distinct_sources, ts.total_mentions, ts.timestamp, ts.metadata_json
		FROM topic_scores ts
		INNER JOIN (
			SELECT topic, MAX(timestamp) AS max_timestamp
			FROM topic_scores
			GROUP BY topic
		) latest
			ON latest.topic = ts.topic AND latest.max_timestamp = ts.timestamp
		ORDER BY ts.score DESC, ts.timestamp DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return 0, 0, fmt.Errorf("query curiosity index scores: %w", err)
	}
	defer rows.Close()

	total := 0.0
	count := 0
	for rows.Next() {
		item, err := scanTopicScore(rows)
		if err != nil {
			return 0, 0, err
		}
		total += item.Score
		count++
	}

	if err := rows.Err(); err != nil {
		return 0, 0, fmt.Errorf("iterate curiosity index scores: %w", err)
	}

	return total, count, nil
}

func (s *Store) SourceCounts(ctx context.Context) ([]models.SourceCount, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT source, COUNT(*)
		FROM normalized_events
		GROUP BY source
		ORDER BY source ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query source counts: %w", err)
	}
	defer rows.Close()

	counts := make([]models.SourceCount, 0)
	for rows.Next() {
		var item models.SourceCount
		if err := rows.Scan(&item.Source, &item.Count); err != nil {
			return nil, fmt.Errorf("scan source count: %w", err)
		}
		counts = append(counts, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate source counts: %w", err)
	}

	return counts, nil
}

func scanTopicScore(scanner interface{ Scan(dest ...any) error }) (models.TopicScore, error) {
	var (
		item         models.TopicScore
		timestampRaw string
		metadataRaw  string
	)

	if err := scanner.Scan(&item.ID, &item.Topic, &item.Score, &item.DistinctSources, &item.TotalMentions, &timestampRaw, &metadataRaw); err != nil {
		return models.TopicScore{}, fmt.Errorf("scan topic score: %w", err)
	}

	if timestampRaw != "" {
		parsed, err := time.Parse(timeLayout, timestampRaw)
		if err != nil {
			return models.TopicScore{}, fmt.Errorf("parse topic score timestamp: %w", err)
		}
		item.Timestamp = parsed.UTC()
	}

	if metadataRaw != "" {
		if err := json.Unmarshal([]byte(metadataRaw), &item.Metadata); err != nil {
			return models.TopicScore{}, fmt.Errorf("decode topic score metadata: %w", err)
		}
	}
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}

	return item, nil
}

func scanTopicSpike(scanner interface{ Scan(dest ...any) error }) (models.TopicSpike, error) {
	var (
		item         models.TopicSpike
		timestampRaw string
		metadataRaw  string
	)

	if err := scanner.Scan(
		&item.ID,
		&item.Topic,
		&item.SpikeType,
		&item.PreviousScore,
		&item.CurrentScore,
		&item.IncreaseRatio,
		&item.IncreaseAbsolute,
		&item.DistinctSources,
		&item.TotalMentions,
		&timestampRaw,
		&metadataRaw,
	); err != nil {
		return models.TopicSpike{}, fmt.Errorf("scan topic spike: %w", err)
	}

	if timestampRaw != "" {
		parsed, err := time.Parse(timeLayout, timestampRaw)
		if err != nil {
			return models.TopicSpike{}, fmt.Errorf("parse topic spike timestamp: %w", err)
		}
		item.Timestamp = parsed.UTC()
	}

	if metadataRaw != "" {
		if err := json.Unmarshal([]byte(metadataRaw), &item.Metadata); err != nil {
			return models.TopicSpike{}, fmt.Errorf("decode topic spike metadata: %w", err)
		}
	}
	if item.Metadata == nil {
		item.Metadata = map[string]any{}
	}

	return item, nil
}

func (s *Store) init() error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
	}
	for _, pragma := range pragmas {
		if _, err := s.db.Exec(pragma); err != nil {
			return fmt.Errorf("initialize sqlite pragma: %w", err)
		}
	}

	statements := strings.Split(schema, ";")
	for _, statement := range statements {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}

		if _, err := s.db.Exec(statement); err != nil {
			return fmt.Errorf("initialize schema statement: %w", err)
		}
	}

	return nil
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || strings.TrimSpace(dir) == "" {
		return nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create sqlite directory: %w", err)
	}

	return nil
}

const timeLayout = "2006-01-02T15:04:05.999999999Z07:00"

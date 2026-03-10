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

func (s *Store) init() error {
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("initialize schema: %w", err)
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

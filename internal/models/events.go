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

type SourceCount struct {
	Source string `json:"source"`
	Count  int64  `json:"count"`
}

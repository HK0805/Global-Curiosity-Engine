package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/signal"
	"strconv"
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
	serviceName     = "event-normalizer"
	outputTopic     = "normalized.events"
	consumerGroupID = "event-normalizer"
)

var inputTopics = []string{
	"raw.wikipedia.edits",
	"raw.reddit.posts",
	"raw.hn.stories",
	"raw.github.events",
	"raw.gdelt.events",
}

type wikipediaEvent struct {
	ID        any    `json:"id"`
	Title     string `json:"title"`
	Timestamp int64  `json:"timestamp"`
	Meta      struct {
		ID  string `json:"id"`
		URI string `json:"uri"`
		DT  string `json:"dt"`
	} `json:"meta"`
	User          string `json:"user"`
	Type          string `json:"type"`
	Wiki          string `json:"wiki"`
	ServerURL     string `json:"server_url"`
	TitleURL      string `json:"title_url"`
	Comment       string `json:"comment"`
	ParsedComment string `json:"parsedcomment"`
}

type redditPost struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Permalink   string  `json:"permalink"`
	Subreddit   string  `json:"subreddit"`
	Author      string  `json:"author"`
	CreatedUTC  float64 `json:"created_utc"`
	NumComments int     `json:"num_comments"`
	Score       int     `json:"score"`
}

type hnStory struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Time        int64  `json:"time"`
	By          string `json:"by"`
	Score       int    `json:"score"`
	Descendants int    `json:"descendants"`
	Type        string `json:"type"`
	Text        string `json:"text"`
}

type githubEvent struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	CreatedAt string `json:"created_at"`
	Repo      struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"repo"`
	Actor struct {
		Login string `json:"login"`
	} `json:"actor"`
	Payload map[string]any `json:"payload"`
}

type gdeltArticle struct {
	URL           string `json:"url"`
	URLMobile     string `json:"url_mobile"`
	Title         string `json:"title"`
	Domain        string `json:"domain"`
	SeenDate      string `json:"seendate"`
	Language      string `json:"language"`
	SourceCountry string `json:"sourcecountry"`
	SocialImage   string `json:"socialimage"`
}

func main() {
	cfg := config.Load()
	logger := logging.New(serviceName)
	reader := internalkafka.NewGroupReader(cfg, inputTopics, consumerGroupID)
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
		"input_topics", strings.Join(inputTopics, ","),
		"output_topic", outputTopic,
	)
	logger.Info("sqlite store ready", "sqlite_path", cfg.SQLitePath)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	processor := newNormalizer(reader, writer, store, logger)

	if err := processor.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("service stopped with error", "error", err)
	}

	logger.Info("service shutting down")
}

type normalizer struct {
	reader *kafkago.Reader
	writer *kafkago.Writer
	store  *appdb.Store
	logger *slog.Logger
}

func newNormalizer(reader *kafkago.Reader, writer *kafkago.Writer, store *appdb.Store, logger *slog.Logger) *normalizer {
	return &normalizer{
		reader: reader,
		writer: writer,
		store:  store,
		logger: logger,
	}
}

func (n *normalizer) Run(ctx context.Context) error {
	n.logger.Info("normalizer ready")

	for {
		message, err := n.reader.ReadMessage(ctx)
		if err != nil {
			return err
		}

		event, err := normalizeMessage(message.Topic, message.Value)
		if err != nil {
			n.logger.Warn("failed to normalize message", "topic", message.Topic, "error", err)
			continue
		}
		if event == nil {
			continue
		}

		payload, err := json.Marshal(event)
		if err != nil {
			n.logger.Warn("failed to encode normalized event", "topic", message.Topic, "error", err)
			continue
		}

		key := event.Source + ":" + event.ID
		if err := n.writer.WriteMessages(ctx, kafkago.Message{
			Key:   []byte(key),
			Value: payload,
			Time:  time.Now().UTC(),
		}); err != nil {
			return fmt.Errorf("write normalized message: %w", err)
		}

		if err := n.store.UpsertNormalizedEvent(ctx, *event); err != nil {
			n.logger.Warn("failed to persist normalized event", "event_id", event.ID, "source", event.Source, "error", err)
			continue
		}

		n.logger.Info("normalized event published", "source_topic", message.Topic, "event_id", event.ID, "source", event.Source)
	}
}

func normalizeMessage(topic string, payload []byte) (*models.NormalizedEvent, error) {
	switch topic {
	case "raw.wikipedia.edits":
		return normalizeWikipedia(payload)
	case "raw.reddit.posts":
		return normalizeReddit(payload)
	case "raw.hn.stories":
		return normalizeHackerNews(payload)
	case "raw.github.events":
		return normalizeGitHub(payload)
	case "raw.gdelt.events":
		return normalizeGDELT(payload)
	default:
		return nil, fmt.Errorf("unsupported topic: %s", topic)
	}
}

func normalizeWikipedia(payload []byte) (*models.NormalizedEvent, error) {
	var event wikipediaEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("decode wikipedia payload: %w", err)
	}

	id := strings.TrimSpace(event.Meta.ID)
	if id == "" && event.ID != nil {
		id = fmt.Sprint(event.ID)
	}
	if id == "" || strings.TrimSpace(event.Title) == "" {
		return nil, nil
	}

	url := strings.TrimSpace(event.Meta.URI)
	if url == "" {
		url = strings.TrimSpace(event.TitleURL)
	}

	ts := time.Now().UTC()
	if event.Timestamp > 0 {
		ts = time.Unix(event.Timestamp, 0).UTC()
	} else if parsed, ok := parseTime(event.Meta.DT, time.RFC3339); ok {
		ts = parsed
	}

	return &models.NormalizedEvent{
		ID:        id,
		Source:    "wikipedia",
		Title:     strings.TrimSpace(event.Title),
		URL:       url,
		Timestamp: ts,
		Metadata: map[string]any{
			"type":           event.Type,
			"user":           event.User,
			"wiki":           event.Wiki,
			"server_url":     event.ServerURL,
			"comment":        event.Comment,
			"parsed_comment": event.ParsedComment,
		},
	}, nil
}

func normalizeReddit(payload []byte) (*models.NormalizedEvent, error) {
	var post redditPost
	if err := json.Unmarshal(payload, &post); err != nil {
		return nil, fmt.Errorf("decode reddit payload: %w", err)
	}

	id := strings.TrimSpace(post.Name)
	if id == "" {
		id = strings.TrimSpace(post.ID)
	}
	if id == "" || strings.TrimSpace(post.Title) == "" {
		return nil, nil
	}

	url := strings.TrimSpace(post.URL)
	if permalink := strings.TrimSpace(post.Permalink); permalink != "" {
		if strings.HasPrefix(permalink, "http") {
			url = permalink
		} else {
			url = "https://www.reddit.com" + permalink
		}
	}

	ts := time.Now().UTC()
	if post.CreatedUTC > 0 {
		ts = time.Unix(int64(post.CreatedUTC), 0).UTC()
	}

	return &models.NormalizedEvent{
		ID:        id,
		Source:    "reddit",
		Title:     strings.TrimSpace(post.Title),
		URL:       url,
		Timestamp: ts,
		Metadata: map[string]any{
			"subreddit":    post.Subreddit,
			"author":       post.Author,
			"score":        post.Score,
			"num_comments": post.NumComments,
		},
	}, nil
}

func normalizeHackerNews(payload []byte) (*models.NormalizedEvent, error) {
	var story hnStory
	if err := json.Unmarshal(payload, &story); err != nil {
		return nil, fmt.Errorf("decode hacker news payload: %w", err)
	}

	if story.ID <= 0 || strings.TrimSpace(story.Title) == "" {
		return nil, nil
	}

	ts := time.Now().UTC()
	if story.Time > 0 {
		ts = time.Unix(story.Time, 0).UTC()
	}

	return &models.NormalizedEvent{
		ID:        strconv.FormatInt(story.ID, 10),
		Source:    "hackernews",
		Title:     strings.TrimSpace(story.Title),
		URL:       strings.TrimSpace(story.URL),
		Timestamp: ts,
		Metadata: map[string]any{
			"author":      story.By,
			"score":       story.Score,
			"descendants": story.Descendants,
			"type":        story.Type,
			"text":        story.Text,
		},
	}, nil
}

func normalizeGitHub(payload []byte) (*models.NormalizedEvent, error) {
	var event githubEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("decode github payload: %w", err)
	}

	id := strings.TrimSpace(event.ID)
	if id == "" {
		return nil, nil
	}

	title := strings.TrimSpace(event.Type)
	if repo := strings.TrimSpace(event.Repo.Name); repo != "" {
		if title != "" {
			title += " on " + repo
		} else {
			title = repo
		}
	}
	if title == "" {
		title = "GitHub event"
	}

	ts := time.Now().UTC()
	if parsed, ok := parseTime(event.CreatedAt, time.RFC3339); ok {
		ts = parsed
	}

	return &models.NormalizedEvent{
		ID:        id,
		Source:    "github",
		Title:     title,
		URL:       strings.TrimSpace(event.Repo.URL),
		Timestamp: ts,
		Metadata: map[string]any{
			"type":    event.Type,
			"repo":    event.Repo.Name,
			"actor":   event.Actor.Login,
			"payload": event.Payload,
		},
	}, nil
}

func normalizeGDELT(payload []byte) (*models.NormalizedEvent, error) {
	var article gdeltArticle
	if err := json.Unmarshal(payload, &article); err != nil {
		return nil, fmt.Errorf("decode gdelt payload: %w", err)
	}

	id := gdeltID(article)
	if id == "" || strings.TrimSpace(article.Title) == "" {
		return nil, nil
	}

	url := strings.TrimSpace(article.URL)
	if url == "" {
		url = strings.TrimSpace(article.URLMobile)
	}

	ts := time.Now().UTC()
	if parsed, ok := parseTime(article.SeenDate, "20060102T150405Z", "20060102T150405"); ok {
		ts = parsed
	}

	return &models.NormalizedEvent{
		ID:        id,
		Source:    "gdelt",
		Title:     strings.TrimSpace(article.Title),
		URL:       url,
		Timestamp: ts,
		Metadata: map[string]any{
			"domain":         article.Domain,
			"language":       article.Language,
			"source_country": article.SourceCountry,
			"social_image":   article.SocialImage,
			"seen_date":      article.SeenDate,
		},
	}, nil
}

func gdeltID(article gdeltArticle) string {
	switch {
	case strings.TrimSpace(article.URL) != "":
		return strings.TrimSpace(article.URL)
	case strings.TrimSpace(article.URLMobile) != "":
		return strings.TrimSpace(article.URLMobile)
	case strings.TrimSpace(article.Domain) != "" && strings.TrimSpace(article.Title) != "":
		return strings.TrimSpace(article.Domain) + "|" + strings.TrimSpace(article.Title)
	case strings.TrimSpace(article.SeenDate) != "" && strings.TrimSpace(article.Title) != "":
		return strings.TrimSpace(article.SeenDate) + "|" + strings.TrimSpace(article.Title)
	default:
		return ""
	}
}

func parseTime(value string, layouts ...string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}

	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), true
		}
	}

	return time.Time{}, false
}

package config

import (
	"os"
	"strings"
)

type Config struct {
	KafkaBroker     string
	SQLitePath      string
	APIPort         string
	GitHubToken     string
	RedditUserAgent string
}

func Load() Config {
	return Config{
		KafkaBroker:     getEnv("KAFKA_BROKER", "kafka:29092"),
		SQLitePath:      getEnv("SQLITE_PATH", "/data/global-curiosity-engine.db"),
		APIPort:         getEnv("API_PORT", "8080"),
		GitHubToken:     strings.TrimSpace(os.Getenv("GITHUB_TOKEN")),
		RedditUserAgent: strings.TrimSpace(os.Getenv("REDDIT_USER_AGENT")),
	}
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

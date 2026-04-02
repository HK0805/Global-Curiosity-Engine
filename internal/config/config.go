package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Config struct {
	KafkaBroker     string
	SQLitePath      string
	APIPort         string
	GitHubToken     string
	RedditUserAgent string
}

var loadEnvOnce sync.Once

func Load() Config {
	loadEnvOnce.Do(loadDotEnv)

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

func loadDotEnv() {
	path := filepath.Join(".", ".env")
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		_ = os.Setenv(key, value)
	}
}

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	appdb "github.com/HK0805/Global-Curiosity-Engine/database"
	"github.com/HK0805/Global-Curiosity-Engine/internal/config"
	"github.com/HK0805/Global-Curiosity-Engine/internal/logging"
)

const (
	serviceName  = "api-server"
	defaultLimit = 50
	maxLimit     = 200
)

func main() {
	cfg := config.Load()
	logger := logging.New(serviceName)
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

	mux := http.NewServeMux()
	handler := newAPIHandler(store, logger)
	mux.HandleFunc("GET /health", handler.handleHealth)
	mux.HandleFunc("GET /events", handler.handleEvents)
	mux.HandleFunc("GET /sources", handler.handleSources)
	mux.HandleFunc("GET /topics/trending", handler.handleTrendingTopics)
	mux.HandleFunc("GET /topics/spikes", handler.handleTopicSpikes)
	mux.HandleFunc("GET /curiosity/index", handler.handleCuriosityIndex)

	server := &http.Server{
		Addr:              ":" + cfg.APIPort,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("service starting", "port", cfg.APIPort, "sqlite_path", cfg.SQLitePath)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server stopped unexpectedly", "error", err)
		}
	}()

	logger.Info(
		"service ready",
		"health_endpoint", "/health",
		"events_endpoint", "/events",
		"sources_endpoint", "/sources",
		"trending_topics_endpoint", "/topics/trending",
		"topic_spikes_endpoint", "/topics/spikes",
		"curiosity_index_endpoint", "/curiosity/index",
	)

	<-ctx.Done()
	logger.Info("service shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}

type apiHandler struct {
	store  *appdb.Store
	logger *slog.Logger
}

func newAPIHandler(store *appdb.Store, logger *slog.Logger) *apiHandler {
	return &apiHandler{store: store, logger: logger}
}

func (h *apiHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	dbStatus := "ok"
	if err := h.store.PingContext(ctx); err != nil {
		dbStatus = "error"
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status":  "degraded",
			"service": serviceName,
			"db":      dbStatus,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": serviceName,
		"db":      dbStatus,
	})
}

func (h *apiHandler) handleEvents(w http.ResponseWriter, r *http.Request) {
	limit, err := parseLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	events, err := h.store.ListNormalizedEvents(ctx, limit)
	if err != nil {
		h.logger.Error("failed to query events", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to query events")
		return
	}

	writeJSON(w, http.StatusOK, events)
}

func (h *apiHandler) handleSources(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	counts, err := h.store.SourceCounts(ctx)
	if err != nil {
		h.logger.Error("failed to query source counts", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to query sources")
		return
	}

	writeJSON(w, http.StatusOK, counts)
}

func (h *apiHandler) handleTrendingTopics(w http.ResponseWriter, r *http.Request) {
	limit, err := parsePhase2Limit(r.URL.Query().Get("limit"), 20)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	scores, err := h.store.ListTrendingTopicScores(ctx, limit)
	if err != nil {
		h.logger.Error("failed to query trending topics", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to query trending topics")
		return
	}

	writeJSON(w, http.StatusOK, scores)
}

func (h *apiHandler) handleTopicSpikes(w http.ResponseWriter, r *http.Request) {
	limit, err := parsePhase2Limit(r.URL.Query().Get("limit"), 20)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	spikes, err := h.store.ListTopicSpikes(ctx, limit)
	if err != nil {
		h.logger.Error("failed to query topic spikes", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to query topic spikes")
		return
	}

	writeJSON(w, http.StatusOK, spikes)
}

func (h *apiHandler) handleCuriosityIndex(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	index, topicsConsidered, err := h.store.CuriosityIndex(ctx, 10)
	if err != nil {
		h.logger.Error("failed to compute curiosity index", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to compute curiosity index")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"curiosity_index":   index,
		"topics_considered": topicsConsidered,
		"generated_at":      time.Now().UTC(),
	})
}

func parsePhase2Limit(raw string, fallback int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	return parseLimit(raw)
}

func parseLimit(raw string) (int, error) {
	if raw == "" {
		return defaultLimit, nil
	}

	limit, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid limit")
	}
	if limit <= 0 {
		return 0, fmt.Errorf("limit must be greater than 0")
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	return limit, nil
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(payload)
}

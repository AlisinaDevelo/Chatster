package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AliSinaDevelo/Chatster/db"
	"github.com/AliSinaDevelo/Chatster/internal/config"
	"github.com/AliSinaDevelo/Chatster/internal/metrics"
	"github.com/AliSinaDevelo/Chatster/internal/telemetry"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.FromEnv()

	shutdownTelemetry, telemetryEnabled, err := telemetry.Setup(context.Background())
	if err != nil {
		slog.Error("telemetry init failed", "err", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			slog.Warn("telemetry shutdown", "err", err)
		}
	}()
	if telemetryEnabled {
		slog.Info("telemetry tracing enabled")
	}

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 30*time.Second)
	repository, err := db.OpenRepository(startupCtx, db.StorageConfig{
		Storage:          cfg.Storage,
		SQLitePath:       cfg.DBPath,
		PostgresDSN:      cfg.PostgresDSN,
		PostgresMinConns: cfg.PostgresMinConns,
		PostgresMaxConns: cfg.PostgresMaxConns,
	})
	cancelStartup()
	if err != nil {
		slog.Error("storage init failed", "err", err, "storage", cfg.Storage)
		os.Exit(1)
	}
	defer func() { _ = repository.Close() }()

	retentionCtx, cancelRetention := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelRetention()

	if cfg.MessageRetentionDays > 0 {
		cutoff := time.Now().UTC().AddDate(0, 0, -cfg.MessageRetentionDays)
		deleted, err := repository.PruneMessagesBeforeContext(retentionCtx, cutoff)
		if err != nil {
			slog.Error("message retention cleanup failed", "err", err, "retention_days", cfg.MessageRetentionDays)
			os.Exit(1)
		}
		metrics.MessagesPruned.Add(float64(deleted))
		slog.Info("message retention cleanup", "deleted_messages", deleted, "retention_days", cfg.MessageRetentionDays)
	}

	if cfg.AuditRetentionDays > 0 {
		cutoff := time.Now().UTC().AddDate(0, 0, -cfg.AuditRetentionDays)
		deleted, err := repository.PruneModerationEventsBeforeContext(retentionCtx, cutoff)
		if err != nil {
			slog.Error("moderation audit retention cleanup failed", "err", err, "retention_days", cfg.AuditRetentionDays)
			os.Exit(1)
		}
		metrics.ModerationEventsPruned.Add(float64(deleted))
		slog.Info("moderation audit retention cleanup", "deleted_events", deleted, "retention_days", cfg.AuditRetentionDays)
	}

	hub := newHub(repository)
	go hub.run()

	handler := mount(cfg, hub, repository)

	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: handler,
	}

	go func() {
		slog.Info("server starting", "addr", cfg.HTTPAddr, "storage", cfg.Storage)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutdown signal")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := shutdownServer(shutdownCtx, srv, hub); err != nil {
		slog.Error("graceful shutdown", "err", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

func shutdownServer(ctx context.Context, srv *http.Server, hub *Hub) error {
	hubErr := hub.Shutdown(ctx)
	serverErr := srv.Shutdown(ctx)
	if hubErr != nil {
		return hubErr
	}
	return serverErr
}

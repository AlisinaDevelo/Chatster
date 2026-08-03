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
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.FromEnv()

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		slog.Error("database init failed", "err", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

	if cfg.MessageRetentionDays > 0 {
		cutoff := time.Now().UTC().AddDate(0, 0, -cfg.MessageRetentionDays)
		deleted, err := database.PruneMessagesBefore(cutoff)
		if err != nil {
			slog.Error("message retention cleanup failed", "err", err, "retention_days", cfg.MessageRetentionDays)
			os.Exit(1)
		}
		metrics.MessagesPruned.Add(float64(deleted))
		slog.Info("message retention cleanup", "deleted_messages", deleted, "retention_days", cfg.MessageRetentionDays)
	}

	hub := newHub(database)
	go hub.run()

	handler := mount(cfg, hub, database)

	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: handler,
	}

	go func() {
		slog.Info("server starting", "addr", cfg.HTTPAddr, "db", cfg.DBPath)
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
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown", "err", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

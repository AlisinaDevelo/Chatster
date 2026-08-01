package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/AliSinaDevelo/Chatster/db"
	"github.com/AliSinaDevelo/Chatster/internal/config"
	"github.com/AliSinaDevelo/Chatster/internal/metrics"
	"github.com/AliSinaDevelo/Chatster/internal/ratelimit"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func newUpgrader() websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		// Origin validated in serveWs (for metrics); allow here after manual check.
		CheckOrigin: func(r *http.Request) bool { return true },
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func newSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("sess_%d", time.Now().UnixNano())
	}
	return "sess_" + hex.EncodeToString(b[:])
}

func serveWs(hub *Hub, cfg config.Config, up websocket.Upgrader, wsRL *ratelimit.WSUpgrade, w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if wsRL != nil && !wsRL.Allow(ip) {
		metrics.WSUpgrades.WithLabelValues("rate_limited").Inc()
		http.Error(w, "too many connections", http.StatusTooManyRequests)
		return
	}
	if !cfg.OriginAllowed(r) {
		metrics.WSUpgrades.WithLabelValues("denied_origin").Inc()
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	conn, err := up.Upgrade(w, r, nil)
	if err != nil {
		metrics.WSUpgrades.WithLabelValues("upgrade_error").Inc()
		slog.Warn("ws upgrade", "err", err)
		return
	}
	metrics.WSUpgrades.WithLabelValues("ok").Inc()
	metrics.ConnectedClients.Inc()

	client := &Client{
		ID:         newSessionID(),
		Conn:       conn,
		Username:   "Anonymous",
		Hub:        hub,
		msgLimiter: newMessageLimiter(cfg),
		send:       make(chan Message, outboundQueueSize),
		done:       make(chan struct{}),
	}

	go client.writeMessages()
	hub.register <- client

	go client.readMessages()
}

func healthHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		dbOK := database.PingContext(ctx) == nil
		status := "ok"
		code := http.StatusOK
		if !dbOK {
			status = "degraded"
			code = http.StatusServiceUnavailable
		}
		dbStatus := "error"
		if dbOK {
			dbStatus = "ok"
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":   status,
			"database": dbStatus,
			"service":  "chatster",
		})
	}
}

func messagesHandler(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		limit := 50
		if raw := r.URL.Query().Get("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 {
				http.Error(w, "limit must be a positive integer", http.StatusBadRequest)
				return
			}
			limit = parsed
			if limit > 100 {
				limit = 100
			}
		}

		messages, err := database.GetRecentMessages(limit)
		if err != nil {
			slog.Warn("list message history", "err", err)
			http.Error(w, "message history unavailable", http.StatusInternalServerError)
			return
		}
		if messages == nil {
			messages = []db.Message{}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"messages": messages,
			"limit":    limit,
		})
	}
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func staticHandler(staticDir string) http.Handler {
	fileServer := http.FileServer(http.Dir(staticDir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleanPath := filepath.Clean(r.URL.Path)
		if cleanPath == "." || cleanPath == string(filepath.Separator) {
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			return
		}

		path := filepath.Join(staticDir, cleanPath)
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}

		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})
}

func mount(cfg config.Config, hub *Hub, database *db.DB) http.Handler {
	r := mux.NewRouter()
	up := newUpgrader()

	var wsRL *ratelimit.WSUpgrade
	if !cfg.DisableWSRateLimit && cfg.WSUpgradeRPS > 0 {
		wsRL = ratelimit.NewWSUpgrade(cfg.WSUpgradeRPS, cfg.WSUpgradeBurst)
	}

	r.Handle("/metrics", promhttp.Handler())

	r.HandleFunc("/health", healthHandler(database))
	r.HandleFunc("/api/messages", messagesHandler(database))

	r.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, cfg, up, wsRL, w, r)
	})

	if cfg.StaticDir != "" {
		r.PathPrefix("/").Handler(staticHandler(cfg.StaticDir))
	} else {
		r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprintf(w, "Chatster API Server")
		})
	}

	r.Use(enableCORS)
	return r
}

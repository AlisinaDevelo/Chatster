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
	"github.com/AliSinaDevelo/Chatster/internal/telemetry"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
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
	return newOpaqueID("sess_")
}

func newInstanceID() string {
	return newOpaqueID("instance_")
}

func newEventID() string {
	return newOpaqueID("event_")
}

func newOpaqueID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano())
	}
	return prefix + hex.EncodeToString(b[:])
}

func serveWs(hub *Hub, cfg config.Config, up websocket.Upgrader, wsRL *ratelimit.WSUpgrade, w http.ResponseWriter, r *http.Request) {
	// The HTTP request context is canceled after Upgrade returns. Preserve its
	// trace values without carrying that cancellation into the socket lifetime.
	traceCtx, traceSpan := telemetry.Start(context.WithoutCancel(r.Context()), "chatster.websocket.session")
	traceTransferred := false
	defer func() {
		if !traceTransferred {
			traceSpan.End()
		}
	}()

	room, err := db.NormalizeRoom(r.URL.Query().Get("room"))
	if err != nil {
		traceSpan.SetAttributes(attribute.String("chatster.websocket.result", "invalid_room"))
		http.Error(w, "invalid room", http.StatusBadRequest)
		return
	}
	traceSpan.SetAttributes(attribute.String("chatster.room", room))
	if hub.isDraining() {
		traceSpan.SetAttributes(attribute.String("chatster.websocket.result", "draining"))
		metrics.WSUpgrades.WithLabelValues("draining").Inc()
		http.Error(w, "server shutting down", http.StatusServiceUnavailable)
		return
	}

	ip := clientIP(r)
	if wsRL != nil && !wsRL.Allow(ip) {
		traceSpan.SetAttributes(attribute.String("chatster.websocket.result", "rate_limited"))
		metrics.WSUpgrades.WithLabelValues("rate_limited").Inc()
		http.Error(w, "too many connections", http.StatusTooManyRequests)
		return
	}
	if !cfg.OriginAllowed(r) {
		traceSpan.SetAttributes(attribute.String("chatster.websocket.result", "denied_origin"))
		metrics.WSUpgrades.WithLabelValues("denied_origin").Inc()
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	conn, err := up.Upgrade(w, r, nil)
	if err != nil {
		telemetry.MarkError(traceSpan)
		traceSpan.SetAttributes(attribute.String("chatster.websocket.result", "upgrade_error"))
		metrics.WSUpgrades.WithLabelValues("upgrade_error").Inc()
		slog.Warn("ws upgrade", "err", err)
		return
	}
	traceSpan.SetAttributes(attribute.String("chatster.websocket.result", "accepted"))
	metrics.WSUpgrades.WithLabelValues("ok").Inc()

	client := &Client{
		ID:         newSessionID(),
		Conn:       conn,
		Username:   "Anonymous",
		Room:       room,
		Hub:        hub,
		msgLimiter: newMessageLimiter(cfg),
		send:       make(chan Message, outboundQueueSize),
		done:       make(chan struct{}),
		traceCtx:   traceCtx,
		traceSpan:  traceSpan,
	}

	go client.writeMessages()
	if !hub.registerClient(client) {
		traceSpan.SetAttributes(attribute.String("chatster.websocket.result", "draining"))
		metrics.WSUpgrades.WithLabelValues("draining").Inc()
		client.closeForShutdown()
		return
	}
	metrics.ConnectedClients.Inc()
	traceTransferred = true

	go client.readMessages()
}

func healthHandler(database db.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		ctx, span := telemetry.Start(ctx, "chatster.storage.ping", attribute.String("chatster.storage.operation", "ping"))
		defer span.End()

		dbOK := database.PingContext(ctx) == nil
		if !dbOK {
			telemetry.MarkError(span)
		}
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

func messagesHandler(database db.Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		room, err := db.NormalizeRoom(r.URL.Query().Get("room"))
		if err != nil {
			http.Error(w, "invalid room", http.StatusBadRequest)
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

		ctx, span := telemetry.Start(
			r.Context(),
			"chatster.storage.history",
			attribute.String("chatster.room", room),
			attribute.Int("chatster.history.limit", limit),
		)
		defer span.End()
		messages, err := database.GetRecentMessagesInRoomContext(ctx, room, limit)
		if err != nil {
			telemetry.MarkError(span)
			slog.Warn("list message history", "err", err)
			http.Error(w, "message history unavailable", http.StatusInternalServerError)
			return
		}
		if messages == nil {
			messages = []db.Message{}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"room":     room,
			"messages": messages,
			"limit":    limit,
		})
	}
}

func traceHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := "unmatched"
		if current := mux.CurrentRoute(r); current != nil {
			if template, err := current.GetPathTemplate(); err == nil && template != "" {
				route = template
			}
		}
		parent := propagation.TraceContext{}.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := telemetry.Start(
			parent,
			"chatster.http.request",
			attribute.String("http.request.method", r.Method),
			attribute.String("http.route", route),
		)
		defer span.End()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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

func mount(cfg config.Config, hub *Hub, database db.Repository) http.Handler {
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
	r.Use(traceHTTP)
	return r
}

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/AliSinaDevelo/Chatster/db"
	"github.com/AliSinaDevelo/Chatster/internal/auth"
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

func serveWs(hub *Hub, cfg config.Config, authService *auth.Service, up websocket.Upgrader, wsRL *ratelimit.WSUpgrade, w http.ResponseWriter, r *http.Request) {
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
	principal, err := authService.Authorize(r, room)
	if err != nil {
		result := "authentication_required"
		if errors.Is(err, auth.ErrRoomDenied) {
			result = "forbidden_room"
		}
		traceSpan.SetAttributes(attribute.String("chatster.websocket.result", result))
		metrics.WSUpgrades.WithLabelValues(result).Inc()
		writeAuthError(w, err)
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

	sessionID := newSessionID()
	username := "Anonymous"
	if principal.Authenticated() {
		sessionID = principal.SessionID
		username = principal.DisplayName
	}
	client := &Client{
		ID:            sessionID,
		UserID:        principal.UserID,
		Conn:          conn,
		Username:      username,
		Authenticated: principal.Authenticated(),
		AuthExpiresAt: principal.ExpiresAt,
		Room:          room,
		Hub:           hub,
		msgLimiter:    newMessageLimiter(cfg),
		send:          make(chan Message, outboundQueueSize),
		done:          make(chan struct{}),
		traceCtx:      traceCtx,
		traceSpan:     traceSpan,
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

func messagesHandler(database db.Repository, authService *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		room, err := db.NormalizeRoom(r.URL.Query().Get("room"))
		if err != nil {
			http.Error(w, "invalid room", http.StatusBadRequest)
			return
		}
		principal, err := authService.Authorize(r, room)
		if err != nil {
			writeAuthError(w, err)
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
			"viewer":   principal,
		})
	}
}

type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	payload := apiError{}
	payload.Error.Code = code
	payload.Error.Message = message
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrRoomDenied):
		writeJSONError(w, http.StatusForbidden, "room_forbidden", "You do not have access to this room.")
	case errors.Is(err, auth.ErrExpiredSession):
		writeJSONError(w, http.StatusUnauthorized, "session_expired", "Your session has expired.")
	default:
		writeJSONError(w, http.StatusUnauthorized, "authentication_required", "A valid session is required.")
	}
}

func sessionHandler(cfg config.Config, authService *auth.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		switch r.Method {
		case http.MethodGet:
			principal, err := authService.Authenticate(r)
			response := map[string]any{
				"mode":          authService.Mode(),
				"authenticated": err == nil && principal.Authenticated(),
			}
			if err == nil {
				response["user"] = principal
			} else if errors.Is(err, auth.ErrExpiredSession) {
				response["reason"] = "session_expired"
			} else {
				response["reason"] = "authentication_required"
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
		case http.MethodPost:
			if authService.Mode() != auth.ModeSession {
				writeJSONError(w, http.StatusConflict, "authentication_disabled", "Session authentication is disabled.")
				return
			}
			if !cfg.OriginAllowed(r) {
				writeJSONError(w, http.StatusForbidden, "origin_forbidden", "The request origin is not allowed.")
				return
			}
			token, ok := bearerToken(r.Header.Get("Authorization"))
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "invalid_credentials", "The access token is invalid.")
				return
			}
			principal, cookie, err := authService.Exchange(token)
			if err != nil {
				if errors.Is(err, auth.ErrInvalidToken) {
					writeJSONError(w, http.StatusUnauthorized, "invalid_credentials", "The access token is invalid.")
					return
				}
				slog.Error("issue session", "err", err)
				writeJSONError(w, http.StatusInternalServerError, "session_unavailable", "A session could not be created.")
				return
			}
			http.SetCookie(w, cookie)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"mode":          authService.Mode(),
				"authenticated": true,
				"user":          principal,
			})
		case http.MethodDelete:
			if !cfg.OriginAllowed(r) {
				writeJSONError(w, http.StatusForbidden, "origin_forbidden", "The request origin is not allowed.")
				return
			}
			http.SetCookie(w, authService.ExpiredCookie())
			w.WriteHeader(http.StatusNoContent)
		default:
			w.Header().Set("Allow", "GET, POST, DELETE")
			writeJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed.")
		}
	}
}

func bearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(strings.TrimSpace(header), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" || len([]byte(token)) > auth.MaxAccessTokenBytes || strings.ContainsAny(token, " \t\r\n") {
		return "", false
	}
	return token, true
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

func enableCORS(cfg config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ws" {
			next.ServeHTTP(w, r)
			return
		}
		origin := r.Header.Get("Origin")
		if origin != "" {
			if !cfg.OriginAllowed(r) {
				writeJSONError(w, http.StatusForbidden, "origin_forbidden", "The request origin is not allowed.")
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Add("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
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
	authService, err := auth.New(auth.Config{Mode: auth.ModeAnonymous})
	if err != nil {
		panic(err)
	}
	return mountWithAuth(cfg, hub, database, authService)
}

func mountWithAuth(cfg config.Config, hub *Hub, database db.Repository, authService *auth.Service) http.Handler {
	r := mux.NewRouter()
	up := newUpgrader()

	var wsRL *ratelimit.WSUpgrade
	if !cfg.DisableWSRateLimit && cfg.WSUpgradeRPS > 0 {
		wsRL = ratelimit.NewWSUpgrade(cfg.WSUpgradeRPS, cfg.WSUpgradeBurst)
	}

	r.Handle("/metrics", promhttp.Handler())

	r.HandleFunc("/health", healthHandler(database))
	r.HandleFunc("/api/messages", messagesHandler(database, authService))
	r.HandleFunc("/api/session", sessionHandler(cfg, authService))

	r.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, cfg, authService, up, wsRL, w, r)
	})

	if cfg.StaticDir != "" {
		r.PathPrefix("/").Handler(staticHandler(cfg.StaticDir))
	} else {
		r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprintf(w, "Chatster API Server")
		})
	}

	r.Use(func(next http.Handler) http.Handler { return enableCORS(cfg, next) })
	r.Use(traceHTTP)
	return r
}

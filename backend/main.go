package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/AliSinaDevelo/Chatster/db"
	"github.com/AliSinaDevelo/Chatster/internal/config"
	"github.com/AliSinaDevelo/Chatster/internal/metrics"
	"github.com/AliSinaDevelo/Chatster/internal/ratelimit"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	maxUsernameRunes = 64
	maxMessageRunes  = 4000
)

// Message represents a chat message
type Message struct {
	ID        int64     `json:"id,omitempty"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// Client represents a connected client
type Client struct {
	ID       string
	Conn     *websocket.Conn
	Username string
	Hub      *Hub
	writeMu  sync.Mutex // gorilla/websocket allows one writer at a time
}

func (c *Client) writeJSON(v any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.Conn.WriteJSON(v)
}

// Hub manages all connected clients
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan Message
	register   chan *Client
	unregister chan *Client
	mutex      sync.Mutex
	database   *db.DB
}

func newHub(database *db.DB) *Hub {
	return &Hub{
		clients: make(map[*Client]bool),
		// Buffered so client read loops are not blocked while the hub writes to their socket (avoids deadlock).
		broadcast:  make(chan Message, 1024),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		database:   database,
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()

			go h.sendMessageHistory(client)

			notification := Message{
				Username: "System",
				Content:  fmt.Sprintf("%s joined the chat", client.Username),
				Type:     "notification",
			}

			_, err := h.database.SaveMessage(notification.Username, notification.Content, notification.Type)
			if err != nil {
				slog.Warn("save join notification", "err", err)
			}

			h.broadcast <- notification

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)

				if client.Username != "Anonymous" {
					notification := Message{
						Username: "System",
						Content:  fmt.Sprintf("%s left the chat", client.Username),
						Type:     "notification",
					}

					_, err := h.database.SaveMessage(notification.Username, notification.Content, notification.Type)
					if err != nil {
						slog.Warn("save leave notification", "err", err)
					}

					h.broadcast <- notification
				}
			}
			h.mutex.Unlock()

		case message := <-h.broadcast:
			h.mutex.Lock()
			for client := range h.clients {
				err := client.writeJSON(message)
				if err != nil {
					slog.Warn("broadcast", "err", err)
					_ = client.Conn.Close()
					delete(h.clients, client)
				}
			}
			h.mutex.Unlock()
		}
	}
}

func (h *Hub) sendMessageHistory(client *Client) {
	messages, err := h.database.GetRecentMessages(50)
	if err != nil {
		slog.Warn("message history", "err", err)
		return
	}

	for _, msg := range messages {
		message := Message{
			Username: msg.Username,
			Content:  msg.Content,
			Type:     msg.Type,
		}
		if err := client.writeJSON(message); err != nil {
			slog.Warn("send history row", "err", err)
			return
		}
	}

	welcome := Message{
		Username: "System",
		Content:  "Welcome to the chat! You can see the last 50 messages.",
		Type:     "notification",
	}
	if err := client.writeJSON(welcome); err != nil {
		slog.Warn("welcome message", "err", err)
	}
}

func newUpgrader() websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		// Origin validated in serveWs (for metrics); allow here after manual check.
		CheckOrigin: func(r *http.Request) bool { return true },
	}
}

func validUsername(s string) bool {
	if s == "" {
		return false
	}
	return utf8.RuneCountInString(s) <= maxUsernameRunes
}

func validMessageBody(s string) bool {
	if s == "" {
		return false
	}
	return utf8.RuneCountInString(s) <= maxMessageRunes
}

func (c *Client) readMessages() {
	defer func() {
		metrics.ConnectedClients.Dec()
		c.Hub.unregister <- c
		_ = c.Conn.Close()
	}()

	for {
		var msg Message
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("read message", "err", err)
			}
			break
		}

		if msg.Type == "username" {
			name := strings.TrimSpace(msg.Content)
			if !validUsername(name) {
				slog.Warn("invalid username rejected")
				continue
			}
			c.Username = name
			continue
		}

		if msg.Type != "message" {
			msg.Type = "message"
		}
		body := strings.TrimSpace(msg.Content)
		if !validMessageBody(body) {
			slog.Warn("invalid message rejected")
			continue
		}
		msg.Content = body
		msg.Username = c.Username

		dbMsg, err := c.Hub.database.SaveMessage(msg.Username, msg.Content, msg.Type)
		if err != nil {
			slog.Warn("save message", "err", err)
		} else {
			msg.ID = dbMsg.ID
			msg.Timestamp = dbMsg.Timestamp
		}

		metrics.MessagesIngested.Inc()
		c.Hub.broadcast <- msg
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
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
		ID:       r.RemoteAddr,
		Conn:     conn,
		Username: "Anonymous",
		Hub:      hub,
	}

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

func mount(cfg config.Config, hub *Hub, database *db.DB) http.Handler {
	r := mux.NewRouter()
	up := newUpgrader()

	var wsRL *ratelimit.WSUpgrade
	if !cfg.DisableWSRateLimit && cfg.WSUpgradeRPS > 0 {
		wsRL = ratelimit.NewWSUpgrade(cfg.WSUpgradeRPS, cfg.WSUpgradeBurst)
	}

	r.Handle("/metrics", promhttp.Handler())

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "Chatster API Server")
	})

	r.HandleFunc("/health", healthHandler(database))

	r.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, cfg, up, wsRL, w, r)
	})

	r.Use(enableCORS)
	return r
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.FromEnv()

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		slog.Error("database init failed", "err", err)
		os.Exit(1)
	}
	defer func() { _ = database.Close() }()

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

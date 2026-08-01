package main

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/AliSinaDevelo/Chatster/db"
	"github.com/AliSinaDevelo/Chatster/internal/metrics"
)

// Hub manages all connected clients.
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

func (h *Hub) disconnectClientLocked(client *Client) {
	if _, ok := h.clients[client]; !ok {
		return
	}
	delete(h.clients, client)
	client.close()

	username := client.username()
	if username == "Anonymous" {
		return
	}

	notification := Message{
		Username: "System",
		Content:  fmt.Sprintf("%s left the chat", username),
		Type:     "notification",
	}

	if _, err := saveMessageObserved(h.database, notification.Username, notification.Content, notification.Type); err != nil {
		slog.Warn("save leave notification", "err", err)
	}

	h.broadcast <- notification
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()

			go h.sendMessageHistory(client)

			username := client.username()
			notification := Message{
				Username: "System",
				Content:  fmt.Sprintf("%s joined the chat", username),
				Type:     "notification",
			}

			_, err := saveMessageObserved(h.database, notification.Username, notification.Content, notification.Type)
			if err != nil {
				slog.Warn("save join notification", "err", err)
			}

			h.broadcast <- notification

		case client := <-h.unregister:
			h.mutex.Lock()
			h.disconnectClientLocked(client)
			h.mutex.Unlock()

		case message := <-h.broadcast:
			started := time.Now()
			h.mutex.Lock()
			for client := range h.clients {
				if !client.enqueue(message) {
					metrics.WSOutboundDrops.WithLabelValues("slow_client").Inc()
					slog.Warn("disconnect slow websocket client")
					h.disconnectClientLocked(client)
				}
			}
			h.mutex.Unlock()
			metrics.BroadcastFanoutDuration.Observe(time.Since(started).Seconds())
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
			ID:        msg.ID,
			Username:  msg.Username,
			Content:   msg.Content,
			Type:      msg.Type,
			Timestamp: msg.Timestamp,
		}
		if !client.enqueue(message) {
			slog.Warn("queue history row")
			return
		}
	}

	welcome := Message{
		Username: "System",
		Content:  "Welcome to the chat! You can see the last 50 messages.",
		Type:     "notification",
	}
	if !client.enqueue(welcome) {
		slog.Warn("queue welcome message")
	}
}

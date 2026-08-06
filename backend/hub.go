package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AliSinaDevelo/Chatster/db"
	"github.com/AliSinaDevelo/Chatster/internal/metrics"
)

// Hub manages all connected clients.
type Hub struct {
	clients      map[*Client]bool
	broadcast    chan Message
	unregister   chan *Client
	done         chan struct{}
	runDone      chan struct{}
	stopOnce     sync.Once
	shutdownOnce sync.Once
	shutdownErr  error
	runStarted   atomic.Bool
	mutex        sync.Mutex
	draining     bool
	drainEvents  chan *Client
	drainPending map[*Client]struct{}
	finished     map[*Client]struct{}
	database     db.Repository
}

func newHub(database db.Repository) *Hub {
	return &Hub{
		clients: make(map[*Client]bool),
		// Buffered so client read loops are not blocked while the hub writes to their socket (avoids deadlock).
		broadcast: make(chan Message, 1024),
		// Unregister remains synchronous while the hub is running so leave notifications retain ordering.
		unregister: make(chan *Client),
		done:       make(chan struct{}),
		runDone:    make(chan struct{}),
		finished:   make(map[*Client]struct{}),
		database:   database,
	}
}

func (h *Hub) isDraining() bool {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	return h.draining
}

func (h *Hub) registerClient(client *Client) bool {
	h.mutex.Lock()
	if h.draining {
		h.mutex.Unlock()
		return false
	}
	h.clients[client] = true
	h.mutex.Unlock()

	go h.sendMessageHistory(client)

	username := client.username()
	room := client.room()
	notification := Message{
		Username: "System",
		Content:  fmt.Sprintf("%s joined the chat", username),
		Type:     "notification",
		Room:     room,
	}

	_, err := saveMessageObservedInRoom(h.database, room, notification.Username, notification.Content, notification.Type)
	if err != nil {
		slog.Warn("save join notification", "err", err)
	}

	h.publish(notification)
	return true
}

func (h *Hub) disconnectClientLocked(client *Client) {
	if _, ok := h.clients[client]; !ok {
		return
	}
	delete(h.clients, client)
	delete(h.finished, client)
	client.close()

	username := client.username()
	if username == "Anonymous" {
		return
	}
	room := client.room()

	notification := Message{
		Username: "System",
		Content:  fmt.Sprintf("%s left the chat", username),
		Type:     "notification",
		Room:     room,
	}

	if _, err := saveMessageObservedInRoom(h.database, room, notification.Username, notification.Content, notification.Type); err != nil {
		slog.Warn("save leave notification", "err", err)
	}

	h.publish(notification)
}

func (h *Hub) publish(message Message) bool {
	select {
	case <-h.done:
		return false
	default:
	}

	select {
	case h.broadcast <- message:
		return true
	case <-h.done:
		return false
	}
}

func (h *Hub) unregisterClient(client *Client) {
	select {
	case <-h.done:
		return
	default:
	}

	select {
	case h.unregister <- client:
	case <-h.done:
	}
}

func (h *Hub) notifyClientFinished(client *Client) {
	h.mutex.Lock()
	h.finished[client] = struct{}{}
	if !h.draining || h.drainEvents == nil {
		h.mutex.Unlock()
		return
	}
	if _, pending := h.drainPending[client]; !pending {
		h.mutex.Unlock()
		return
	}
	delete(h.drainPending, client)
	events := h.drainEvents
	remaining := len(h.drainPending)
	h.mutex.Unlock()

	metrics.WSDrainClientsRemaining.Set(float64(remaining))
	events <- client
}

func (h *Hub) run() {
	h.runStarted.Store(true)
	defer close(h.runDone)

	for {
		select {
		case <-h.done:
			return
		default:
		}

		select {
		case <-h.done:
			return
		case client := <-h.unregister:
			h.mutex.Lock()
			h.disconnectClientLocked(client)
			h.mutex.Unlock()

		case message := <-h.broadcast:
			started := time.Now()
			h.mutex.Lock()
			for client := range h.clients {
				if message.Room != "" && client.room() != message.Room {
					continue
				}
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

func (h *Hub) stop() {
	h.stopOnce.Do(func() {
		close(h.done)
	})
}

func (h *Hub) Shutdown(ctx context.Context) error {
	h.shutdownOnce.Do(func() {
		h.shutdownErr = h.drain(ctx)
	})
	return h.shutdownErr
}

func (h *Hub) drain(ctx context.Context) error {
	started := time.Now()
	metrics.WSDrainStarted.Inc()

	h.mutex.Lock()
	h.draining = true
	clients := make([]*Client, 0, len(h.clients))
	for client := range h.clients {
		if _, done := h.finished[client]; done {
			delete(h.finished, client)
			continue
		}
		clients = append(clients, client)
	}
	h.clients = make(map[*Client]bool)
	h.drainEvents = make(chan *Client, len(clients))
	h.drainPending = make(map[*Client]struct{}, len(clients))
	for _, client := range clients {
		h.drainPending[client] = struct{}{}
	}
	events := h.drainEvents
	h.mutex.Unlock()

	metrics.WSDrainClientsRemaining.Set(float64(len(clients)))
	slog.Info("websocket drain started", "clients", len(clients), "close_code", shutdownCloseCode)

	var closeGroup sync.WaitGroup
	for _, client := range clients {
		closeGroup.Add(1)
		go func(client *Client) {
			defer closeGroup.Done()
			client.closeForShutdown()
		}(client)
	}
	closeGroup.Wait()

	pending := make(map[*Client]struct{}, len(clients))
	for _, client := range clients {
		pending[client] = struct{}{}
	}
	remaining := len(pending)
	forcedCloses := 0
	for remaining > 0 {
		select {
		case client := <-events:
			if _, ok := pending[client]; !ok {
				continue
			}
			delete(pending, client)
			remaining--
		case <-ctx.Done():
			forcedCloses = remaining
			metrics.WSDrainForcedCloses.Add(float64(forcedCloses))
			slog.Warn("websocket drain deadline reached", "remaining_clients", remaining, "forced_closes", forcedCloses)
		}
		if forcedCloses > 0 {
			break
		}
	}

	h.stop()
	if h.runStarted.Load() {
		<-h.runDone
	}

	duration := time.Since(started)
	metrics.WSDrainDuration.Observe(duration.Seconds())
	slog.Info("websocket drain completed", "duration_seconds", duration.Seconds(), "remaining_clients", remaining, "forced_closes", forcedCloses)
	if forcedCloses > 0 {
		return ctx.Err()
	}
	return nil
}

func (h *Hub) sendMessageHistory(client *Client) {
	room := client.room()
	ctx, cancel := context.WithTimeout(context.Background(), storageOperationTimeout)
	defer cancel()
	messages, err := h.database.GetRecentMessagesInRoomContext(ctx, room, 50)
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
			Room:      msg.Room,
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
		Room:     room,
	}
	if !client.enqueue(welcome) {
		slog.Warn("queue welcome message")
	}
}

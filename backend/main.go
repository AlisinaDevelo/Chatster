package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/AliSinaDevelo/Chatster/db"
	"github.com/AliSinaDevelo/Chatster/internal/config"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
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
		clients:    make(map[*Client]bool),
		broadcast:  make(chan Message),
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

			// Send recent message history to the new client
			go h.sendMessageHistory(client)

			// Notify about new user
			notification := Message{
				Username: "System",
				Content:  fmt.Sprintf("%s joined the chat", client.Username),
				Type:     "notification",
			}

			// Save the notification
			_, err := h.database.SaveMessage(notification.Username, notification.Content, notification.Type)
			if err != nil {
				log.Printf("Error saving notification: %v", err)
			}

			h.broadcast <- notification

		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)

				// Don't notify if username is not set
				if client.Username != "Anonymous" {
					notification := Message{
						Username: "System",
						Content:  fmt.Sprintf("%s left the chat", client.Username),
						Type:     "notification",
					}

					// Save the notification
					_, err := h.database.SaveMessage(notification.Username, notification.Content, notification.Type)
					if err != nil {
						log.Printf("Error saving notification: %v", err)
					}

					h.broadcast <- notification
				}
			}
			h.mutex.Unlock()

		case message := <-h.broadcast:
			h.mutex.Lock()
			for client := range h.clients {
				err := client.Conn.WriteJSON(message)
				if err != nil {
					log.Printf("Error broadcasting message: %v", err)
					client.Conn.Close()
					delete(h.clients, client)
				}
			}
			h.mutex.Unlock()
		}
	}
}

func (h *Hub) sendMessageHistory(client *Client) {
	// Get recent messages from the database
	messages, err := h.database.GetRecentMessages(50)
	if err != nil {
		log.Printf("Error retrieving message history: %v", err)
		return
	}

	// Send message history to the client
	for _, msg := range messages {
		message := Message{
			Username: msg.Username,
			Content:  msg.Content,
			Type:     msg.Type,
		}
		err := client.Conn.WriteJSON(message)
		if err != nil {
			log.Printf("Error sending message history: %v", err)
			return
		}
	}

	// Send a welcome message
	welcome := Message{
		Username: "System",
		Content:  "Welcome to the chat! You can see the last 50 messages.",
		Type:     "notification",
	}
	client.Conn.WriteJSON(welcome)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func (c *Client) readMessages() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	for {
		var msg Message
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Error reading message: %v", err)
			}
			break
		}

		// If this is the first message, it might be a username setup
		if msg.Type == "username" {
			c.Username = msg.Content
			continue
		}

		// Add username to the message
		msg.Username = c.Username

		// Save message to database
		dbMsg, err := c.Hub.database.SaveMessage(msg.Username, msg.Content, msg.Type)
		if err != nil {
			log.Printf("Error saving message: %v", err)
		} else {
			// Update message with database fields
			msg.ID = dbMsg.ID
			msg.Timestamp = dbMsg.Timestamp
		}

		c.Hub.broadcast <- msg
	}
}

func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	client := &Client{
		ID:       r.RemoteAddr,
		Conn:     conn,
		Username: "Anonymous", // Default username
		Hub:      hub,
	}

	hub.register <- client

	go client.readMessages()
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

func main() {
	cfg := config.FromEnv()

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Create router
	r := mux.NewRouter()

	// Initialize the hub with the database
	hub := newHub(database)
	go hub.run()

	// Set up routes
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Chatster API Server")
	})

	r.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
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
	})

	r.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	// Add CORS middleware
	r.Use(enableCORS)

	log.Printf("Chatster listening on %s (db %s)", cfg.HTTPAddr, cfg.DBPath)
	log.Fatal(http.ListenAndServe(cfg.HTTPAddr, r))
}

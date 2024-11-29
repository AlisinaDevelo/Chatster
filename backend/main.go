package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// Message represents a chat message
type Message struct {
	Username string `json:"username"`
	Content  string `json:"content"`
	Type     string `json:"type"`
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
}

func newHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()
			// Notify about new user
			h.broadcast <- Message{
				Username: "System",
				Content:  fmt.Sprintf("%s joined the chat", client.Username),
				Type:     "notification",
			}
		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close := Message{
					Username: "System",
					Content:  fmt.Sprintf("%s left the chat", client.Username),
					Type:     "notification",
				}
				h.broadcast <- close
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
	r := mux.NewRouter()
	hub := newHub()
	go hub.run()

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Chatster API Server")
	})

	r.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	r.Use(enableCORS)

	fmt.Println("Chatster Server v0.1.0 - Starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

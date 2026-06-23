package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type outboundMessage struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type result struct {
	URL               string  `json:"url"`
	Clients           int     `json:"clients"`
	MessagesPerClient int     `json:"messages_per_client"`
	TotalSent         int64   `json:"total_sent"`
	TotalReceived     int64   `json:"total_received"`
	Errors            int64   `json:"errors"`
	DurationMS        int64   `json:"duration_ms"`
	SendRatePerSecond float64 `json:"send_rate_per_second"`
}

func main() {
	url := flag.String("url", "ws://127.0.0.1:8080/ws", "Chatster WebSocket URL")
	clients := flag.Int("clients", 10, "number of concurrent WebSocket clients")
	messages := flag.Int("messages", 10, "messages sent by each client")
	prefix := flag.String("username-prefix", "load", "username prefix")
	connectTimeout := flag.Duration("connect-timeout", 5*time.Second, "WebSocket dial timeout")
	drainFor := flag.Duration("drain-for", 2*time.Second, "time to keep reading after sends complete")
	flag.Parse()

	if *clients < 1 {
		log.Fatal("clients must be at least 1")
	}
	if *messages < 1 {
		log.Fatal("messages must be at least 1")
	}

	dialer := websocket.Dialer{HandshakeTimeout: *connectTimeout}
	conns := make([]*websocket.Conn, 0, *clients)
	for i := 0; i < *clients; i++ {
		conn, _, err := dialer.Dial(*url, nil)
		if err != nil {
			closeAll(conns)
			log.Fatalf("dial client %d: %v", i, err)
		}
		conns = append(conns, conn)
	}
	defer closeAll(conns)

	var received atomic.Int64
	var errors atomic.Int64
	var readers sync.WaitGroup
	for _, conn := range conns {
		readers.Add(1)
		go func(c *websocket.Conn) {
			defer readers.Done()
			for {
				if _, _, err := c.ReadMessage(); err != nil {
					return
				}
				received.Add(1)
			}
		}(conn)
	}

	start := time.Now()
	var sent atomic.Int64
	var senders sync.WaitGroup
	for i, conn := range conns {
		senders.Add(1)
		go func(clientID int, c *websocket.Conn) {
			defer senders.Done()

			username := fmt.Sprintf("%s-%03d", *prefix, clientID)
			if err := c.WriteJSON(outboundMessage{Type: "username", Content: username}); err != nil {
				errors.Add(1)
				return
			}

			for j := 0; j < *messages; j++ {
				content := fmt.Sprintf("load message client=%d seq=%d", clientID, j)
				if err := c.WriteJSON(outboundMessage{Type: "message", Content: content}); err != nil {
					errors.Add(1)
					return
				}
				sent.Add(1)
			}
		}(i, conn)
	}
	senders.Wait()
	duration := time.Since(start)

	time.Sleep(*drainFor)
	closeAll(conns)
	readers.Wait()

	r := result{
		URL:               *url,
		Clients:           *clients,
		MessagesPerClient: *messages,
		TotalSent:         sent.Load(),
		TotalReceived:     received.Load(),
		Errors:            errors.Load(),
		DurationMS:        duration.Milliseconds(),
	}
	if duration > 0 {
		r.SendRatePerSecond = float64(r.TotalSent) / duration.Seconds()
	}

	encoded, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(encoded))

	if r.Errors > 0 {
		log.Fatalf("load run completed with %d send errors", r.Errors)
	}
}

func closeAll(conns []*websocket.Conn) {
	for _, conn := range conns {
		_ = conn.Close()
	}
}

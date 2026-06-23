package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type outboundMessage struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type inboundMessage struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type result struct {
	URL               string `json:"url"`
	Clients           int    `json:"clients"`
	MessagesPerClient int    `json:"messages_per_client"`
	SendInterval      string `json:"send_interval"`
	TotalSent         int64  `json:"total_sent"`
	TotalReceived     int64  `json:"total_received"`
	Delivered         int64  `json:"delivered"`
	Errors            int64  `json:"errors"`
	SendDurationMS    int64  `json:"send_duration_ms"`
	// DeliveredThroughput is matched broadcast deliveries per second over the
	// window from the first send to the last in-scope delivery. It reflects the
	// rate at which the server actually fanned messages out to connected clients.
	DeliveredThroughput float64 `json:"delivered_throughput_per_second"`
	// Round-trip latency from send to delivery, measured across every matched
	// delivery (each message is delivered to every connected client).
	LatencyP50MS  float64 `json:"latency_p50_ms"`
	LatencyP95MS  float64 `json:"latency_p95_ms"`
	LatencyP99MS  float64 `json:"latency_p99_ms"`
	LatencyMaxMS  float64 `json:"latency_max_ms"`
	LatencyMeanMS float64 `json:"latency_mean_ms"`
}

func main() {
	url := flag.String("url", "ws://127.0.0.1:8080/ws", "Chatster WebSocket URL")
	clients := flag.Int("clients", 10, "number of concurrent WebSocket clients")
	messages := flag.Int("messages", 10, "messages sent by each client")
	prefix := flag.String("username-prefix", "load", "username prefix")
	connectTimeout := flag.Duration("connect-timeout", 5*time.Second, "WebSocket dial timeout")
	sendInterval := flag.Duration("send-interval", 0, "pause between each client's messages (0 = as fast as possible)")
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
	var lastRecvNanos atomic.Int64
	// Each reader collects its own latency samples to avoid lock contention on
	// the hot path; slices are merged after the readers stop.
	latencyByReader := make([][]int64, len(conns))
	var readers sync.WaitGroup
	for idx, conn := range conns {
		readers.Add(1)
		go func(slot int, c *websocket.Conn) {
			defer readers.Done()
			var samples []int64
			for {
				_, data, err := c.ReadMessage()
				if err != nil {
					latencyByReader[slot] = samples
					return
				}
				received.Add(1)
				now := time.Now().UnixNano()
				var msg inboundMessage
				if json.Unmarshal(data, &msg) != nil {
					continue
				}
				var sentNanos, cid, seq int64
				if n, _ := fmt.Sscanf(msg.Content, "wsload %d %d %d", &sentNanos, &cid, &seq); n == 3 {
					samples = append(samples, now-sentNanos)
					lastRecvNanos.Store(now)
				}
			}
		}(idx, conn)
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
				content := fmt.Sprintf("wsload %d %d %d", time.Now().UnixNano(), clientID, j)
				if err := c.WriteJSON(outboundMessage{Type: "message", Content: content}); err != nil {
					errors.Add(1)
					return
				}
				sent.Add(1)
				if *sendInterval > 0 && j < *messages-1 {
					time.Sleep(*sendInterval)
				}
			}
		}(i, conn)
	}
	senders.Wait()
	sendDuration := time.Since(start)

	time.Sleep(*drainFor)
	closeAll(conns)
	readers.Wait()

	latencies := make([]int64, 0)
	for _, s := range latencyByReader {
		latencies = append(latencies, s...)
	}

	r := result{
		URL:               *url,
		Clients:           *clients,
		MessagesPerClient: *messages,
		SendInterval:      sendInterval.String(),
		TotalSent:         sent.Load(),
		TotalReceived:     received.Load(),
		Delivered:         int64(len(latencies)),
		Errors:            errors.Load(),
		SendDurationMS:    sendDuration.Milliseconds(),
	}

	if last := lastRecvNanos.Load(); last > 0 && len(latencies) > 0 {
		window := time.Duration(last - start.UnixNano())
		if window > 0 {
			r.DeliveredThroughput = float64(len(latencies)) / window.Seconds()
		}
	}

	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		var sum int64
		for _, v := range latencies {
			sum += v
		}
		r.LatencyP50MS = msFromNanos(percentile(latencies, 0.50))
		r.LatencyP95MS = msFromNanos(percentile(latencies, 0.95))
		r.LatencyP99MS = msFromNanos(percentile(latencies, 0.99))
		r.LatencyMaxMS = msFromNanos(latencies[len(latencies)-1])
		r.LatencyMeanMS = msFromNanos(sum / int64(len(latencies)))
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

func percentile(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p * float64(len(sorted)-1))
	return sorted[idx]
}

func msFromNanos(ns int64) float64 {
	return float64(ns) / float64(time.Millisecond)
}

func closeAll(conns []*websocket.Conn) {
	for _, conn := range conns {
		_ = conn.Close()
	}
}

package main

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AliSinaDevelo/Chatster/db"
	"github.com/AliSinaDevelo/Chatster/internal/config"
	"github.com/gorilla/websocket"
)

func testStack(t *testing.T) (cfg config.Config, database *db.DB, hub *Hub, cleanup func()) {
	t.Helper()
	cfg = config.Config{
		DBPath:             filepath.Join(t.TempDir(), "integration.db"),
		DisableWSRateLimit: true,
		WSUpgradeRPS:       5,
		WSUpgradeBurst:     10,
	}
	d, err := db.Open(cfg.DBPath)
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	h := newHub(d)
	go h.run()
	return cfg, d, h, func() { _ = d.Close() }
}

func TestHealth_OK(t *testing.T) {
	cfg, database, hub, cleanup := testStack(t)
	defer cleanup()

	srv := httptest.NewServer(mount(cfg, hub, database))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestHealth_DegradedWhenDBClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "closed.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(healthHandler(database))
	defer srv.Close()
	_ = database.Close()

	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", resp.StatusCode)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	cfg, database, hub, cleanup := testStack(t)
	defer cleanup()

	srv := httptest.NewServer(mount(cfg, hub, database))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "chatster_") {
		t.Fatalf("metrics body missing chatster_ prefix: %s", body[:min(200, len(body))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestWebSocketUsernameAndMessage(t *testing.T) {
	cfg, database, hub, cleanup := testStack(t)
	defer cleanup()

	srv := httptest.NewServer(mount(cfg, hub, database))
	defer srv.Close()

	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	c, resp, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("dial: %v (resp=%v)", err, resp)
	}
	defer func() { _ = c.Close() }()

	received := make(chan Message, 64)
	go func() {
		for {
			var m Message
			if err := c.ReadJSON(&m); err != nil {
				return
			}
			received <- m
		}
	}()

	time.Sleep(300 * time.Millisecond)

	if err := c.WriteJSON(Message{Type: "username", Content: "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteJSON(Message{Type: "message", Content: "hi"}); err != nil {
		t.Fatal(err)
	}

	deadline := time.After(15 * time.Second)
	found := false
	for !found {
		select {
		case m := <-received:
			if m.Type == "message" && m.Content == "hi" && m.Username == "alice" {
				found = true
			}
		case <-deadline:
			t.Fatal("timeout waiting for echoed chat message")
		}
	}
}

func TestWebSocketRejectsDeniedOrigin(t *testing.T) {
	cfg, database, hub, cleanup := testStack(t)
	defer cleanup()
	cfg.AllowedOrigins = []string{"https://chatster.example"}

	srv := httptest.NewServer(mount(cfg, hub, database))
	defer srv.Close()

	headers := http.Header{}
	headers.Set("Origin", "https://evil.example")
	c, resp, err := websocket.DefaultDialer.Dial(wsURL(srv), headers)
	if err == nil {
		_ = c.Close()
		t.Fatal("expected denied origin to fail websocket dial")
	}
	if resp == nil {
		t.Fatal("expected HTTP response for denied origin")
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %d", resp.StatusCode)
	}
}

func TestWebSocketUpgradeRateLimit(t *testing.T) {
	cfg, database, hub, cleanup := testStack(t)
	defer cleanup()
	cfg.DisableWSRateLimit = false
	cfg.WSUpgradeRPS = 0.001
	cfg.WSUpgradeBurst = 1

	srv := httptest.NewServer(mount(cfg, hub, database))
	defer srv.Close()

	first, resp, err := websocket.DefaultDialer.Dial(wsURL(srv), nil)
	if err != nil {
		t.Fatalf("first dial: %v (resp=%v)", err, resp)
	}
	defer func() { _ = first.Close() }()

	second, resp, err := websocket.DefaultDialer.Dial(wsURL(srv), nil)
	if err == nil {
		_ = second.Close()
		t.Fatal("expected second dial to be rate limited")
	}
	if resp == nil {
		t.Fatal("expected HTTP response for rate-limited dial")
	}
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d", resp.StatusCode)
	}
}

func TestWebSocketRejectedUsernameKeepsAnonymousIdentity(t *testing.T) {
	cfg, database, hub, cleanup := testStack(t)
	defer cleanup()

	srv := httptest.NewServer(mount(cfg, hub, database))
	defer srv.Close()

	c := mustDialWS(t, srv)
	defer func() { _ = c.Close() }()

	if err := c.WriteJSON(Message{Type: "username", Content: ""}); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteJSON(Message{Type: "message", Content: "still here"}); err != nil {
		t.Fatal(err)
	}

	msg := readMatchingMessage(t, c, func(m Message) bool {
		return m.Type == "message" && m.Content == "still here"
	})
	if msg.Username != "Anonymous" {
		t.Fatalf("invalid username should keep anonymous identity, got %q", msg.Username)
	}
}

func TestWebSocketRejectsInvalidMessages(t *testing.T) {
	cfg, database, hub, cleanup := testStack(t)
	defer cleanup()

	srv := httptest.NewServer(mount(cfg, hub, database))
	defer srv.Close()

	c := mustDialWS(t, srv)
	defer func() { _ = c.Close() }()

	if err := c.WriteJSON(Message{Type: "username", Content: "alice"}); err != nil {
		t.Fatal(err)
	}

	if err := c.WriteJSON(Message{Type: "message", Content: "   "}); err != nil {
		t.Fatal(err)
	}
	oversizedBody := strings.Repeat("x", maxMessageRunes+1)
	if err := c.WriteJSON(Message{Type: "message", Content: oversizedBody}); err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)
	if got := messageContentCount(t, database, ""); got != 0 {
		t.Fatalf("empty message should not persist, got %d", got)
	}
	if got := messageContentCount(t, database, oversizedBody); got != 0 {
		t.Fatalf("oversized message should not persist, got %d", got)
	}
}

func TestWebSocketClosesOversizedFrames(t *testing.T) {
	cfg, database, hub, cleanup := testStack(t)
	defer cleanup()

	srv := httptest.NewServer(mount(cfg, hub, database))
	defer srv.Close()

	c := mustDialWS(t, srv)
	defer func() { _ = c.Close() }()

	oversizedPayload := strings.Repeat("x", maxWebSocketReadBytes+1)
	if err := c.WriteMessage(websocket.TextMessage, []byte(oversizedPayload)); err != nil {
		return
	}

	if err := c.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				t.Fatal("timed out waiting for oversized frame to close websocket")
			}
			return
		}
	}
}

func TestWebSocketCoercesClientMessageType(t *testing.T) {
	cfg, database, hub, cleanup := testStack(t)
	defer cleanup()

	srv := httptest.NewServer(mount(cfg, hub, database))
	defer srv.Close()

	c := mustDialWS(t, srv)
	defer func() { _ = c.Close() }()

	if err := c.WriteJSON(Message{Type: "username", Content: "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteJSON(Message{Type: "notification", Content: "spoof"}); err != nil {
		t.Fatal(err)
	}

	msg := readMatchingMessage(t, c, func(m Message) bool {
		return m.Content == "spoof"
	})
	if msg.Type != "message" {
		t.Fatalf("client-supplied non-message type should be coerced, got %q", msg.Type)
	}
}

func TestWebSocketMessageRateLimit(t *testing.T) {
	cfg, database, hub, cleanup := testStack(t)
	defer cleanup()
	cfg.MessageRPS = 0.001
	cfg.MessageBurst = 1

	srv := httptest.NewServer(mount(cfg, hub, database))
	defer srv.Close()

	c := mustDialWS(t, srv)
	defer func() { _ = c.Close() }()

	if err := c.WriteJSON(Message{Type: "username", Content: "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteJSON(Message{Type: "message", Content: "first"}); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteJSON(Message{Type: "message", Content: "second"}); err != nil {
		t.Fatal(err)
	}

	readMatchingMessage(t, c, func(m Message) bool {
		return m.Type == "notification" && strings.Contains(m.Content, "too quickly")
	})

	if got := messageContentCount(t, database, "first"); got != 1 {
		t.Fatalf("first message should persist once, got %d", got)
	}
	if got := messageContentCount(t, database, "second"); got != 0 {
		t.Fatalf("rate-limited message should not persist, got %d", got)
	}

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), `chatster_chat_messages_rejected_total{reason="rate_limited"}`) {
		t.Fatalf("metrics missing rate_limited rejection counter")
	}
}

func wsURL(srv *httptest.Server) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
}

func mustDialWS(t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	c, resp, err := websocket.DefaultDialer.Dial(wsURL(srv), nil)
	if err != nil {
		t.Fatalf("dial: %v (resp=%v)", err, resp)
	}
	return c
}

func readMatchingMessage(t *testing.T, c *websocket.Conn, match func(Message) bool) Message {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err := c.SetReadDeadline(deadline); err != nil {
			t.Fatal(err)
		}
		var m Message
		if err := c.ReadJSON(&m); err != nil {
			t.Fatalf("read matching message: %v", err)
		}
		if match(m) {
			return m
		}
	}
}

func messageContentCount(t *testing.T, database *db.DB, content string) int {
	t.Helper()
	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM messages WHERE content = ?", content).Scan(&count); err != nil {
		t.Fatalf("count messages by content: %v", err)
	}
	return count
}

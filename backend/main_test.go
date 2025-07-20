package main

import (
	"io"
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

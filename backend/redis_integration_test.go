package main

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AliSinaDevelo/Chatster/db"
	"github.com/AliSinaDevelo/Chatster/internal/broker"
	"github.com/AliSinaDevelo/Chatster/internal/config"
	"github.com/AliSinaDevelo/Chatster/internal/events"
	"github.com/gorilla/websocket"
)

func TestRedisFanoutAcrossInstances(t *testing.T) {
	redisURL := strings.TrimSpace(os.Getenv("CHATSTER_REDIS_TEST_URL"))
	if redisURL == "" {
		t.Skip("set CHATSTER_REDIS_TEST_URL to run the Docker-backed Redis integration")
	}

	namespace := fmt.Sprintf("integration-%d", time.Now().UnixNano())
	sharedDBPath := filepath.Join(t.TempDir(), "shared-history.db")
	databaseA, err := db.Open(sharedDBPath)
	if err != nil {
		t.Fatalf("open instance A database: %v", err)
	}
	databaseB, err := db.Open(sharedDBPath)
	if err != nil {
		_ = databaseA.Close()
		t.Fatalf("open instance B database: %v", err)
	}

	fanoutA, err := broker.NewRedis(broker.Config{URL: redisURL, Namespace: namespace, InstanceID: "instance-a"})
	if err != nil {
		_ = databaseA.Close()
		_ = databaseB.Close()
		t.Fatalf("connect instance A broker: %v", err)
	}
	fanoutB, err := broker.NewRedis(broker.Config{URL: redisURL, Namespace: namespace, InstanceID: "instance-b"})
	if err != nil {
		_ = fanoutA.Close()
		_ = databaseA.Close()
		_ = databaseB.Close()
		t.Fatalf("connect instance B broker: %v", err)
	}

	cfg := config.Config{
		DisableWSRateLimit:      true,
		DisableMessageRateLimit: true,
	}
	hubA := newHub(databaseA, fanoutA)
	hubB := newHub(databaseB, fanoutB)
	go hubA.run()
	go hubB.run()
	hubA.startBroker()
	hubB.startBroker()
	serverA := httptest.NewServer(mount(cfg, hubA, databaseA))
	serverB := httptest.NewServer(mount(cfg, hubB, databaseB))
	defer func() {
		serverA.Close()
		serverB.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := hubA.Shutdown(ctx); err != nil {
			t.Errorf("shutdown instance A: %v", err)
		}
		if err := hubB.Shutdown(ctx); err != nil {
			t.Errorf("shutdown instance B: %v", err)
		}
		_ = databaseA.Close()
		_ = databaseB.Close()
	}()

	awaitRedisReady(t, fanoutA.Ready())
	awaitRedisReady(t, fanoutB.Ready())

	generalA := mustRedisTestWS(t, serverA, "general")
	generalB := mustRedisTestWS(t, serverB, "general")
	engineeringB := mustRedisTestWS(t, serverB, "engineering")
	defer func() {
		closeRedisTestWS(generalA)
		closeRedisTestWS(generalB)
		closeRedisTestWS(engineeringB)
	}()

	writeRedisTestMessage(t, generalA, Message{Type: "username", Content: "alice"})
	writeRedisTestMessage(t, generalB, Message{Type: "username", Content: "bob"})
	writeRedisTestMessage(t, engineeringB, Message{Type: "username", Content: "eng"})

	sameRoomContent := fmt.Sprintf("same-room-%d", time.Now().UnixNano())
	writeRedisTestMessage(t, generalA, Message{Type: "message", Content: sameRoomContent})
	if !redisTestReadsContent(generalB, sameRoomContent, 3*time.Second) {
		t.Fatalf("instance B did not receive same-room message")
	}
	if redisTestReadsContent(engineeringB, sameRoomContent, 500*time.Millisecond) {
		t.Fatalf("same-room message crossed into engineering")
	}
	closeRedisTestWS(engineeringB)

	duplicateContent := fmt.Sprintf("duplicate-%d", time.Now().UnixNano())
	event, err := events.New("duplicate-event", "general", fanoutA.InstanceID(), events.Message{
		ID:       time.Now().UnixNano(),
		Username: "alice",
		Content:  duplicateContent,
		Type:     "message",
	}, time.Now().UTC())
	if err != nil {
		t.Fatalf("build duplicate event: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	if err := fanoutA.Publish(ctx, event); err != nil {
		cancel()
		t.Fatalf("publish first duplicate event: %v", err)
	}
	if err := fanoutA.Publish(ctx, event); err != nil {
		cancel()
		t.Fatalf("publish second duplicate event: %v", err)
	}
	cancel()
	if !redisTestReadsContent(generalB, duplicateContent, 3*time.Second) {
		t.Fatalf("instance B did not receive duplicate test event")
	}
	if redisTestReadsContent(generalB, duplicateContent, 500*time.Millisecond) {
		t.Fatalf("duplicate event was delivered twice")
	}
	closeRedisTestWS(generalB)
	generalB = mustRedisTestWS(t, serverB, "general")

	reconnectContent := fmt.Sprintf("reconnect-%d", time.Now().UnixNano())
	writeRedisTestMessage(t, generalA, Message{Type: "message", Content: reconnectContent})
	if !redisTestReadsContent(generalB, reconnectContent, 3*time.Second) {
		t.Fatalf("instance B did not receive reconnect history seed")
	}
	closeRedisTestWS(generalB)
	generalB = mustRedisTestWS(t, serverB, "general")
	if !redisTestReadsContent(generalB, reconnectContent, 3*time.Second) {
		t.Fatalf("instance B did not recover shared history after reconnect")
	}
}

func awaitRedisReady(t *testing.T, ready <-chan struct{}) {
	t.Helper()
	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		t.Fatal("Redis broker did not become ready")
	}
}

func mustRedisTestWS(t *testing.T, server *httptest.Server, room string) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?room=" + room
	connection, response, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial Redis test room %q: %v (response=%v)", room, err, response)
	}
	return connection
}

func writeRedisTestMessage(t *testing.T, connection *websocket.Conn, message Message) {
	t.Helper()
	if err := connection.WriteJSON(message); err != nil {
		t.Fatalf("write Redis test message: %v", err)
	}
}

func closeRedisTestWS(connection *websocket.Conn) {
	if connection == nil {
		return
	}
	_ = connection.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseGoingAway, "test complete"),
		time.Now().Add(time.Second),
	)
	_ = connection.Close()
}

func redisTestReadsContent(connection *websocket.Conn, content string, timeout time.Duration) bool {
	_ = connection.SetReadDeadline(time.Now().Add(timeout))
	defer func() { _ = connection.SetReadDeadline(time.Time{}) }()
	for {
		var message Message
		if err := connection.ReadJSON(&message); err != nil {
			return false
		}
		if message.Content == content {
			return true
		}
	}
}

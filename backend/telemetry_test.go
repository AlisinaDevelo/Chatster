package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AliSinaDevelo/Chatster/internal/telemetry"
	"github.com/gorilla/websocket"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTracingSpansAvoidMessageContent(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	telemetry.SetTracerProvider(provider)
	t.Cleanup(func() {
		telemetry.SetTracerProvider(nil)
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown tracer provider: %v", err)
		}
	})

	cfg, database, hub, cleanup := testStack(t)
	defer cleanup()

	srv := httptest.NewServer(mount(cfg, hub, database))
	defer srv.Close()

	const secret = "message body that must stay out of spans"
	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?room=engineering"
	conn, resp, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("dial: %v (resp=%v)", err, resp)
	}
	defer func() { _ = conn.Close() }()

	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteJSON(Message{Type: "message", Content: secret}); err != nil {
		t.Fatal(err)
	}
	for {
		var message Message
		if err := conn.ReadJSON(&message); err != nil {
			t.Fatalf("read echoed message: %v", err)
		}
		if message.Type == "message" && message.Content == secret {
			break
		}
	}

	response, err := http.Get(srv.URL + "/api/messages?room=engineering&limit=10")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("history status %d", response.StatusCode)
	}

	_ = conn.Close()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if hasSpan(exporter.GetSpans(), "chatster.websocket.session") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := provider.ForceFlush(context.Background()); err != nil {
		t.Fatal(err)
	}

	wanted := map[string]bool{
		"chatster.http.request":                false,
		"chatster.storage.history":             false,
		"chatster.websocket.session":           false,
		"chatster.storage.save_message":        false,
		"chatster.websocket.broadcast.enqueue": false,
	}
	for _, span := range exporter.GetSpans() {
		if _, ok := wanted[span.Name]; ok {
			wanted[span.Name] = true
		}
		for _, attr := range span.Attributes {
			if strings.Contains(attr.Value.AsString(), secret) {
				t.Fatalf("span %q contains message content in attribute %q", span.Name, attr.Key)
			}
		}
	}
	for name, found := range wanted {
		if !found {
			t.Errorf("missing span %q; got %#v", name, exporter.GetSpans())
		}
	}
}

func hasSpan(spans tracetest.SpanStubs, name string) bool {
	for _, span := range spans {
		if span.Name == name {
			return true
		}
	}
	return false
}

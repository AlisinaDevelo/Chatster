package events

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFixtureIsValid(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "room-message-v1.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var envelope Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if err := envelope.Validate(); err != nil {
		t.Fatalf("validate fixture: %v", err)
	}

	channel, err := Channel("production", envelope.Room)
	if err != nil {
		t.Fatalf("channel: %v", err)
	}
	if channel != "chatster:v1:production:room:general" {
		t.Fatalf("channel: got %q", channel)
	}
}

func TestNewNormalizesTimestampToUTC(t *testing.T) {
	timestamp := time.Date(2026, time.August, 6, 12, 0, 0, 0, time.FixedZone("test", 2*60*60))
	envelope, err := New("event-1", "general", "instance-a", Message{
		ID:       42,
		Username: "alice",
		Content:  "hello",
		Type:     "message",
	}, timestamp)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if envelope.Timestamp.Location() != time.UTC {
		t.Fatalf("timestamp location: got %v want UTC", envelope.Timestamp.Location())
	}
	if !envelope.Timestamp.Equal(timestamp) {
		t.Fatalf("timestamp: got %s want %s", envelope.Timestamp, timestamp)
	}
}

func TestValidateRejectsUnsafeOrNondurableEvents(t *testing.T) {
	base := Envelope{
		Schema:         Schema,
		Version:        Version,
		EventID:        "event-1",
		Room:           "general",
		OriginInstance: "instance-a",
		Timestamp:      time.Now().UTC(),
		Message: Message{
			ID:       42,
			Username: "alice",
			Content:  "hello",
			Type:     "message",
		},
	}

	tests := []struct {
		name   string
		mutate func(*Envelope)
	}{
		{name: "unknown schema", mutate: func(e *Envelope) { e.Schema = "other" }},
		{name: "noncanonical room", mutate: func(e *Envelope) { e.Room = "General" }},
		{name: "unsafe origin", mutate: func(e *Envelope) { e.OriginInstance = "prod:1" }},
		{name: "zero message ID", mutate: func(e *Envelope) { e.Message.ID = 0 }},
		{name: "username notification", mutate: func(e *Envelope) { e.Message.Type = "username" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event := base
			test.mutate(&event)
			if err := event.Validate(); err == nil {
				t.Fatal("Validate should fail")
			}
		})
	}
}

func TestChannelCanonicalizesNamespaceAndRoom(t *testing.T) {
	channel, err := Channel(" Production ", " Engineering ")
	if err != nil {
		t.Fatalf("Channel: %v", err)
	}
	if channel != "chatster:v1:production:room:engineering" {
		t.Fatalf("channel: got %q", channel)
	}
}

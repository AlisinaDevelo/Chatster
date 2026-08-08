package events

import (
	"fmt"
	"strings"
	"time"

	"github.com/AliSinaDevelo/Chatster/db"
)

const (
	// Schema identifies the room-message event family on the wire.
	Schema = "chatster.room-message"
	// Version identifies the compatible envelope schema version.
	Version         = 1
	maxTokenRunes   = 64
	maxEventIDRunes = 128
)

// Message is the storage-neutral payload carried by a cross-instance event.
// The envelope owns the room and persistence timestamp.
type Message struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Content  string `json:"content"`
	Type     string `json:"type"`
}

// Envelope is the versioned wire contract for room fan-out.
type Envelope struct {
	Schema         string    `json:"schema"`
	Version        int       `json:"version"`
	EventID        string    `json:"event_id"`
	Room           string    `json:"room"`
	OriginInstance string    `json:"origin_instance"`
	Timestamp      time.Time `json:"timestamp"`
	Message        Message   `json:"message"`
}

// New builds a validated event using a persistence timestamp in UTC.
func New(eventID, room, originInstance string, message Message, timestamp time.Time) (Envelope, error) {
	envelope := Envelope{
		Schema:         Schema,
		Version:        Version,
		EventID:        eventID,
		Room:           room,
		OriginInstance: originInstance,
		Timestamp:      timestamp.UTC(),
		Message:        message,
	}
	if err := envelope.Validate(); err != nil {
		return Envelope{}, err
	}
	return envelope, nil
}

// Validate rejects malformed or unsafe envelopes before they reach the hub.
func (e Envelope) Validate() error {
	if e.Schema != Schema {
		return fmt.Errorf("unsupported event schema %q", e.Schema)
	}
	if e.Version != Version {
		return fmt.Errorf("unsupported event version %d", e.Version)
	}
	if e.EventID == "" || e.EventID != strings.TrimSpace(e.EventID) {
		return fmt.Errorf("event ID must be a non-empty token")
	}
	if len([]rune(e.EventID)) > maxEventIDRunes {
		return fmt.Errorf("event ID must be at most %d characters", maxEventIDRunes)
	}
	if e.Timestamp.IsZero() {
		return fmt.Errorf("event timestamp is required")
	}
	if e.Timestamp.Location() != time.UTC {
		return fmt.Errorf("event timestamp must be UTC")
	}

	room, err := db.NormalizeRoom(e.Room)
	if err != nil || room != e.Room {
		if err != nil {
			return fmt.Errorf("invalid event room: %w", err)
		}
		return fmt.Errorf("event room must be canonical: got %q want %q", e.Room, room)
	}
	originInstance, err := normalizeToken(e.OriginInstance, "origin instance")
	if err != nil {
		return err
	}
	if originInstance != e.OriginInstance {
		return fmt.Errorf("origin instance must be canonical: got %q want %q", e.OriginInstance, originInstance)
	}
	if e.Message.ID <= 0 {
		return fmt.Errorf("event message ID must be positive")
	}
	if strings.TrimSpace(e.Message.Username) == "" {
		return fmt.Errorf("event message username is required")
	}
	if e.Message.Content == "" {
		return fmt.Errorf("event message content is required")
	}
	if e.Message.Type == "" || e.Message.Type == "username" {
		return fmt.Errorf("event message type must be durable")
	}
	return nil
}

// Channel returns the exact Redis channel for a canonical room and namespace.
func Channel(namespace, room string) (string, error) {
	namespace, err := normalizeToken(namespace, "broker namespace")
	if err != nil {
		return "", err
	}
	room, err = db.NormalizeRoom(room)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("chatster:v%d:%s:room:%s", Version, namespace, room), nil
}

// ChannelPattern returns the subscription pattern for every room in a namespace.
func ChannelPattern(namespace string) (string, error) {
	namespace, err := normalizeToken(namespace, "broker namespace")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("chatster:v%d:%s:room:*", Version, namespace), nil
}

// ValidateInstanceID validates the origin identifier used for loop prevention.
func ValidateInstanceID(instanceID string) error {
	_, err := normalizeToken(instanceID, "instance ID")
	return err
}

func normalizeToken(value, field string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	if len([]rune(value)) > maxTokenRunes {
		return "", fmt.Errorf("%s must be at most %d characters", field, maxTokenRunes)
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return "", fmt.Errorf("%s may contain only letters, digits, hyphens, and underscores", field)
	}
	return value, nil
}

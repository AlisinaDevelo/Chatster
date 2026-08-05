package main

import (
	"time"
	"unicode/utf8"

	"github.com/AliSinaDevelo/Chatster/db"
	"github.com/AliSinaDevelo/Chatster/internal/metrics"
)

const (
	maxUsernameRunes = 64
	maxMessageRunes  = 4000
)

// Message represents a chat message.
type Message struct {
	ID        int64     `json:"id,omitempty"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	Type      string    `json:"type"`
	Room      string    `json:"room,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

func validUsername(s string) bool {
	if s == "" {
		return false
	}
	return utf8.RuneCountInString(s) <= maxUsernameRunes
}

func validMessageBody(s string) bool {
	if s == "" {
		return false
	}
	return utf8.RuneCountInString(s) <= maxMessageRunes
}

func saveMessageObserved(database *db.DB, username, content, msgType string) (*db.Message, error) {
	return saveMessageObservedInRoom(database, db.DefaultRoom, username, content, msgType)
}

func saveMessageObservedInRoom(database *db.DB, room, username, content, msgType string) (*db.Message, error) {
	started := time.Now()
	msg, err := database.SaveMessageInRoom(room, username, content, msgType)
	result := "ok"
	if err != nil {
		result = "error"
	}
	metrics.MessagePersistDuration.WithLabelValues(result).Observe(time.Since(started).Seconds())
	return msg, err
}

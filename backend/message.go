package main

import (
	"context"
	"time"
	"unicode/utf8"

	"github.com/AliSinaDevelo/Chatster/db"
	"github.com/AliSinaDevelo/Chatster/internal/metrics"
)

const (
	maxUsernameRunes        = 64
	maxMessageRunes         = 4000
	storageOperationTimeout = 5 * time.Second
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

func saveMessageObserved(database db.Repository, username, content, msgType string) (*db.Message, error) {
	return saveMessageObservedInRoom(database, db.DefaultRoom, username, content, msgType)
}

func saveMessageObservedInRoom(database db.Repository, room, username, content, msgType string) (*db.Message, error) {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), storageOperationTimeout)
	defer cancel()
	msg, err := database.SaveMessageInRoomContext(ctx, room, username, content, msgType)
	result := "ok"
	if err != nil {
		result = "error"
	}
	metrics.MessagePersistDuration.WithLabelValues(result).Observe(time.Since(started).Seconds())
	return msg, err
}

package db

import (
	"path/filepath"
	"testing"
)

func TestNewAndSaveMessage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	msg, err := database.SaveMessage("alice", "hello", "message")
	if err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}
	if msg.ID == 0 {
		t.Fatal("expected non-zero id")
	}
	if msg.Username != "alice" || msg.Content != "hello" || msg.Type != "message" {
		t.Fatalf("unexpected message: %+v", msg)
	}
}

func TestSaveMessageSkipsUsernameTypePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	_, err = database.SaveMessage("anon", "bob", "username")
	if err != nil {
		t.Fatalf("SaveMessage username type: %v", err)
	}

	rows, err := database.Query("SELECT COUNT(*) FROM messages")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("expected count row")
	}
	var n int
	if err := rows.Scan(&n); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if n != 0 {
		t.Fatalf("username handshake should not persist, got count %d", n)
	}
}

func TestGetRecentMessagesOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	for _, text := range []string{"first", "second", "third"} {
		if _, err := database.SaveMessage("u", text, "message"); err != nil {
			t.Fatalf("SaveMessage %q: %v", text, err)
		}
	}

	msgs, err := database.GetRecentMessages(10)
	if err != nil {
		t.Fatalf("GetRecentMessages: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("want 3 messages, got %d", len(msgs))
	}
	if got := []string{msgs[0].Content, msgs[1].Content, msgs[2].Content}; got[0] != "first" || got[1] != "second" || got[2] != "third" {
		t.Fatalf("chronological order: %+v", got)
	}
}

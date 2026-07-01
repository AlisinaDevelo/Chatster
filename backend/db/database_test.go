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

func TestOpenConfiguresSQLiteForSingleNodeRuntime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.db")
	database, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	stats := database.Stats()
	if stats.MaxOpenConnections != 1 {
		t.Fatalf("MaxOpenConnections: got %d want 1", stats.MaxOpenConnections)
	}

	var journalMode string
	if err := database.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode: got %q want %q", journalMode, "wal")
	}

	var foreignKeys int
	if err := database.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys: got %d want 1", foreignKeys)
	}

	var busyTimeout int
	if err := database.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("busy_timeout: %v", err)
	}
	if busyTimeout != sqliteBusyTimeoutMS {
		t.Fatalf("busy_timeout: got %d want %d", busyTimeout, sqliteBusyTimeoutMS)
	}
}

func TestOpenAppliesSchemaMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "migrations.db")
	database, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	var name string
	if err := database.QueryRow("SELECT name FROM schema_migrations WHERE version = 1").Scan(&name); err != nil {
		t.Fatalf("schema migration row: %v", err)
	}
	if name != "create_messages" {
		t.Fatalf("migration name: got %q want %q", name, "create_messages")
	}

	if err := database.QueryRow("SELECT name FROM schema_migrations WHERE version = 2").Scan(&name); err != nil {
		t.Fatalf("schema migration row: %v", err)
	}
	if name != "create_moderation_audit_log" {
		t.Fatalf("migration name: got %q want %q", name, "create_moderation_audit_log")
	}
}

func TestOpenMigrationsAreIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "idempotent.db")
	database, err := Open(path)
	if err != nil {
		t.Fatalf("Open first: %v", err)
	}
	if _, err := database.SaveMessage("alice", "still here", "message"); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close first: %v", err)
	}

	database, err = Open(path)
	if err != nil {
		t.Fatalf("Open second: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	var migrationRows int
	if err := database.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 1").Scan(&migrationRows); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if migrationRows != 1 {
		t.Fatalf("migration should be recorded once, got %d", migrationRows)
	}

	var messageRows int
	if err := database.QueryRow("SELECT COUNT(*) FROM messages WHERE content = ?", "still here").Scan(&messageRows); err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if messageRows != 1 {
		t.Fatalf("existing messages should survive reopen, got %d", messageRows)
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
	defer func() { _ = rows.Close() }()
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

func TestSaveModerationEventPersistsAuditMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.db")
	database, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	event, err := database.SaveModerationEvent("sess_abc", "alice", "invalid_body", "   ")
	if err != nil {
		t.Fatalf("SaveModerationEvent: %v", err)
	}
	if event.ID == 0 {
		t.Fatal("expected non-zero id")
	}
	if event.Timestamp.IsZero() {
		t.Fatal("expected timestamp")
	}

	var sessionID, username, reason string
	var contentLength int
	if err := database.QueryRow(`
SELECT session_id, username, reason, content_length
FROM moderation_audit_log
WHERE id = ?`, event.ID).Scan(&sessionID, &username, &reason, &contentLength); err != nil {
		t.Fatalf("query audit row: %v", err)
	}
	if sessionID != "sess_abc" || username != "alice" || reason != "invalid_body" {
		t.Fatalf("unexpected audit metadata: session=%q username=%q reason=%q", sessionID, username, reason)
	}
	if contentLength != 3 {
		t.Fatalf("content_length: got %d want 3", contentLength)
	}
}

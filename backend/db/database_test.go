package db

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
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

	if err := database.QueryRow("SELECT name FROM schema_migrations WHERE version = 3").Scan(&name); err != nil {
		t.Fatalf("schema migration row: %v", err)
	}
	if name != "index_message_timestamp" {
		t.Fatalf("migration name: got %q want %q", name, "index_message_timestamp")
	}

	if err := database.QueryRow("SELECT name FROM schema_migrations WHERE version = 4").Scan(&name); err != nil {
		t.Fatalf("schema migration row: %v", err)
	}
	if name != "index_moderation_audit_timestamp" {
		t.Fatalf("migration name: got %q want %q", name, "index_moderation_audit_timestamp")
	}

	if err := database.QueryRow("SELECT name FROM schema_migrations WHERE version = 5").Scan(&name); err != nil {
		t.Fatalf("schema migration row: %v", err)
	}
	if name != "add_message_rooms" {
		t.Fatalf("migration name: got %q want %q", name, "add_message_rooms")
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

func TestGetRecentMessagesUsesIDForTimestampTies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tie-order.db")
	database, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	first, err := database.SaveMessage("u", "first", "message")
	if err != nil {
		t.Fatalf("SaveMessage first: %v", err)
	}
	second, err := database.SaveMessage("u", "second", "message")
	if err != nil {
		t.Fatalf("SaveMessage second: %v", err)
	}

	if _, err := database.Exec(
		"UPDATE messages SET timestamp = ? WHERE id IN (?, ?)",
		"2026-01-01 00:00:00",
		first.ID,
		second.ID,
	); err != nil {
		t.Fatalf("set equal timestamps: %v", err)
	}

	messages, err := database.GetRecentMessages(10)
	if err != nil {
		t.Fatalf("GetRecentMessages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("want 2 messages, got %d", len(messages))
	}
	if messages[0].ID != first.ID || messages[1].ID != second.ID {
		t.Fatalf("equal-timestamp order: got IDs %d, %d want %d, %d", messages[0].ID, messages[1].ID, first.ID, second.ID)
	}
}

func TestNormalizeRoom(t *testing.T) {
	for _, test := range []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "default", input: "", want: DefaultRoom},
		{name: "trim and lowercase", input: " Engineering ", want: "engineering"},
		{name: "hyphen and underscore", input: "team_ops-2", want: "team_ops-2"},
		{name: "leading punctuation", input: "_hidden", wantErr: true},
		{name: "embedded space", input: "team ops", wantErr: true},
		{name: "slash", input: "team/ops", wantErr: true},
		{name: "too long", input: strings.Repeat("a", maxRoomRunes+1), wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := NormalizeRoom(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("NormalizeRoom(%q) should fail", test.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeRoom(%q): %v", test.input, err)
			}
			if got != test.want {
				t.Fatalf("NormalizeRoom(%q): got %q want %q", test.input, got, test.want)
			}
		})
	}
}

func TestMessagesAreScopedToRooms(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rooms.db")
	database, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	if _, err := database.SaveMessage("alice", "general message", "message"); err != nil {
		t.Fatalf("SaveMessage: %v", err)
	}
	if _, err := database.SaveMessageInRoom("engineering", "bob", "room message", "message"); err != nil {
		t.Fatalf("SaveMessageInRoom: %v", err)
	}

	general, err := database.GetRecentMessagesInRoom(DefaultRoom, 10)
	if err != nil {
		t.Fatalf("general history: %v", err)
	}
	if len(general) != 1 || general[0].Content != "general message" || general[0].Room != DefaultRoom {
		t.Fatalf("unexpected general history: %#v", general)
	}

	engineering, err := database.GetRecentMessagesInRoom("engineering", 10)
	if err != nil {
		t.Fatalf("engineering history: %v", err)
	}
	if len(engineering) != 1 || engineering[0].Content != "room message" || engineering[0].Room != "engineering" {
		t.Fatalf("unexpected engineering history: %#v", engineering)
	}
}

func TestPruneMessagesBeforeRemovesExpiredMessages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "retention.db")
	database, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	oldTimestamp := time.Now().UTC().AddDate(0, 0, -3)
	recentTimestamp := time.Now().UTC().Add(-time.Hour)
	for _, row := range []struct {
		content   string
		timestamp time.Time
	}{
		{content: "expired", timestamp: oldTimestamp},
		{content: "recent", timestamp: recentTimestamp},
	} {
		if _, err := database.Exec(`
INSERT INTO messages(username, content, type, timestamp)
VALUES(?, ?, ?, ?)`, "alice", row.content, "message", row.timestamp); err != nil {
			t.Fatalf("insert %q: %v", row.content, err)
		}
	}

	deleted, err := database.PruneMessagesBefore(time.Now().UTC().AddDate(0, 0, -1))
	if err != nil {
		t.Fatalf("PruneMessagesBefore: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted: got %d want 1", deleted)
	}

	var remaining int
	if err := database.QueryRow("SELECT COUNT(*) FROM messages").Scan(&remaining); err != nil {
		t.Fatalf("count remaining messages: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("remaining messages: got %d want 1", remaining)
	}
}

func TestPruneModerationEventsBeforeRemovesExpiredEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit-retention.db")
	database, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	oldTimestamp := time.Now().UTC().AddDate(0, 0, -120)
	recentTimestamp := time.Now().UTC().Add(-time.Hour)
	for _, row := range []struct {
		reason    string
		timestamp time.Time
	}{
		{reason: "old_rejection", timestamp: oldTimestamp},
		{reason: "recent_rejection", timestamp: recentTimestamp},
	} {
		if _, err := database.Exec(`
INSERT INTO moderation_audit_log(
	session_id, username, reason, content_preview, content_length, timestamp
) VALUES(?, ?, ?, ?, ?, ?)`, "sess_test", "alice", row.reason, "blocked", 7, row.timestamp); err != nil {
			t.Fatalf("insert %q: %v", row.reason, err)
		}
	}

	deleted, err := database.PruneModerationEventsBefore(time.Now().UTC().AddDate(0, 0, -30))
	if err != nil {
		t.Fatalf("PruneModerationEventsBefore: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted: got %d want 1", deleted)
	}

	var remaining int
	if err := database.QueryRow("SELECT COUNT(*) FROM moderation_audit_log").Scan(&remaining); err != nil {
		t.Fatalf("count remaining audit events: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("remaining audit events: got %d want 1", remaining)
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

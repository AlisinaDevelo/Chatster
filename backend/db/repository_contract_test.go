package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRepositoryContractSQLite(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "contract.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	runRepositoryContract(t, database, func(t *testing.T, event *ModerationEvent) {
		var (
			sessionID      string
			userID         string
			username       string
			reason         string
			contentPreview string
			contentLength  int
		)
		if err := database.QueryRow(`
SELECT session_id, user_id, username, reason, content_preview, content_length
FROM moderation_audit_log WHERE id = ?`, event.ID).
			Scan(&sessionID, &userID, &username, &reason, &contentPreview, &contentLength); err != nil {
			t.Fatalf("read SQLite audit event: %v", err)
		}
		if sessionID != event.SessionID || userID != event.UserID || username != event.Username || reason != event.Reason || contentPreview != event.ContentPreview || contentLength != event.ContentLength {
			t.Fatalf("SQLite audit row mismatch: got %q/%q/%q/%q/%q/%d, want %#v", sessionID, userID, username, reason, contentPreview, contentLength, event)
		}
	})
}

func TestRepositoryContractPostgres(t *testing.T) {
	database := openPostgresTestRepository(t)

	runRepositoryContract(t, database, func(t *testing.T, event *ModerationEvent) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var (
			sessionID      string
			userID         string
			username       string
			reason         string
			contentPreview string
			contentLength  int
		)
		if err := database.pool.QueryRow(ctx, `
SELECT session_id, user_id, username, reason, content_preview, content_length
FROM moderation_audit_log WHERE id = $1`, event.ID).
			Scan(&sessionID, &userID, &username, &reason, &contentPreview, &contentLength); err != nil {
			t.Fatalf("read Postgres audit event: %v", err)
		}
		if sessionID != event.SessionID || userID != event.UserID || username != event.Username || reason != event.Reason || contentPreview != event.ContentPreview || contentLength != event.ContentLength {
			t.Fatalf("Postgres audit row mismatch: got %q/%q/%q/%q/%q/%d, want %#v", sessionID, userID, username, reason, contentPreview, contentLength, event)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := database.runMigrations(ctx); err != nil {
		t.Fatalf("idempotent Postgres migrations: %v", err)
	}
}

func TestPostgresRepositoryUsesIDAsHistoryTieBreaker(t *testing.T) {
	database := openPostgresTestRepository(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	room := fmt.Sprintf("tie-%d", time.Now().UnixNano()%1_000_000_000)
	first, err := database.SaveMessageInRoomContext(ctx, room, "alice", "first", "message")
	if err != nil {
		t.Fatalf("save first message: %v", err)
	}
	second, err := database.SaveMessageInRoomContext(ctx, room, "bob", "second", "message")
	if err != nil {
		t.Fatalf("save second message: %v", err)
	}
	fixedTimestamp := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := database.pool.Exec(ctx, "UPDATE messages SET timestamp = $1 WHERE id IN ($2, $3)", fixedTimestamp, first.ID, second.ID); err != nil {
		t.Fatalf("set equal timestamps: %v", err)
	}

	history, err := database.GetRecentMessagesInRoomContext(ctx, room, 10)
	if err != nil {
		t.Fatalf("read equal-timestamp history: %v", err)
	}
	if len(history) != 2 || history[0].ID != first.ID || history[1].ID != second.ID {
		t.Fatalf("equal-timestamp order: got %#v want IDs %d, %d", history, first.ID, second.ID)
	}
}

func openPostgresTestRepository(t *testing.T) *PostgresDB {
	t.Helper()
	dsn := strings.TrimSpace(testPostgresDSN())
	if dsn == "" {
		t.Skip("CHATSTER_POSTGRES_TEST_DSN is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	database, err := OpenPostgres(ctx, dsn, 2, 4)
	if err != nil {
		t.Fatalf("open Postgres test repository: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func testPostgresDSN() string {
	return strings.TrimSpace(os.Getenv("CHATSTER_POSTGRES_TEST_DSN"))
}

func runRepositoryContract(t *testing.T, repository Repository, verifyAudit func(*testing.T, *ModerationEvent)) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	room := fmt.Sprintf("contract-%d", time.Now().UnixNano()%1_000_000_000)
	otherRoom := room + "-other"
	first, err := repository.SaveMessageForUserInRoomContext(ctx, room, "usr_alice", "alice", "first", "message")
	if err != nil {
		t.Fatalf("save first message: %v", err)
	}
	second, err := repository.SaveMessageInRoomContext(ctx, room, "bob", "second", "message")
	if err != nil {
		t.Fatalf("save second message: %v", err)
	}
	if first.ID < 1 || second.ID < 1 || first.Timestamp.IsZero() || second.Timestamp.IsZero() {
		t.Fatalf("persisted messages need IDs and timestamps: first=%#v second=%#v", first, second)
	}
	if first.UserID != "usr_alice" || second.UserID != "" {
		t.Fatalf("stable user identity should be optional and preserved: first=%#v second=%#v", first, second)
	}

	if _, err := repository.SaveMessageInRoomContext(ctx, otherRoom, "carol", "other room", "message"); err != nil {
		t.Fatalf("save other-room message: %v", err)
	}
	history, err := repository.GetRecentMessagesInRoomContext(ctx, room, 10)
	if err != nil {
		t.Fatalf("read room history: %v", err)
	}
	if len(history) != 2 || history[0].Content != "first" || history[1].Content != "second" {
		t.Fatalf("room history should be chronological and isolated: %#v", history)
	}
	if history[0].Room != room || history[1].Room != room {
		t.Fatalf("room history contains the wrong room: %#v", history)
	}
	if history[0].UserID != "usr_alice" || history[1].UserID != "" {
		t.Fatalf("room history lost stable user identity: %#v", history)
	}

	otherHistory, err := repository.GetRecentMessagesInRoomContext(ctx, otherRoom, 10)
	if err != nil {
		t.Fatalf("read other-room history: %v", err)
	}
	if len(otherHistory) != 1 || otherHistory[0].Content != "other room" {
		t.Fatalf("other-room history: %#v", otherHistory)
	}

	usernameMessage, err := repository.SaveMessageInRoomContext(ctx, room, "alice", "Alice", "username")
	if err != nil {
		t.Fatalf("save username message: %v", err)
	}
	if usernameMessage.ID != 0 {
		t.Fatalf("username messages must not receive a persisted ID: %#v", usernameMessage)
	}
	history, err = repository.GetRecentMessagesInRoomContext(ctx, room, 10)
	if err != nil {
		t.Fatalf("read history after username message: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("username message was persisted: %#v", history)
	}

	content := strings.Repeat("é", maxAuditPreviewRunes+4)
	event, err := repository.SaveModerationEventForUserContext(ctx, "sess_contract", "usr_alice", "alice", "invalid_body", content)
	if err != nil {
		t.Fatalf("save moderation event: %v", err)
	}
	if event.ContentLength != maxAuditPreviewRunes+4 || len([]rune(event.ContentPreview)) != maxAuditPreviewRunes {
		t.Fatalf("audit metadata: %#v", event)
	}
	if event.UserID != "usr_alice" {
		t.Fatalf("audit event lost stable user identity: %#v", event)
	}
	verifyAudit(t, event)

	deletedMessages, err := repository.PruneMessagesBeforeContext(ctx, time.Now().UTC().Add(time.Second))
	if err != nil {
		t.Fatalf("prune messages: %v", err)
	}
	if deletedMessages < 3 {
		t.Fatalf("prune should remove contract messages, deleted %d", deletedMessages)
	}
	deletedEvents, err := repository.PruneModerationEventsBeforeContext(ctx, time.Now().UTC().Add(time.Second))
	if err != nil {
		t.Fatalf("prune moderation events: %v", err)
	}
	if deletedEvents < 1 {
		t.Fatalf("prune should remove contract audit event, deleted %d", deletedEvents)
	}

	if err := repository.PingContext(ctx); err != nil {
		t.Fatalf("repository health ping: %v", err)
	}
}

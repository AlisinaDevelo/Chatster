package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	_ "github.com/mattn/go-sqlite3" // Driver registers with database/sql as "sqlite3".
)

const sqliteBusyTimeoutMS = 5000

const maxAuditPreviewRunes = 256

const (
	// DefaultRoom is the room used when a client does not select one.
	DefaultRoom  = "general"
	maxRoomRunes = 32
)

type migration struct {
	version int
	name    string
	sql     string
}

var migrations = []migration{
	{
		version: 1,
		name:    "create_messages",
		sql: `
CREATE TABLE IF NOT EXISTS messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL,
	content TEXT NOT NULL,
	type TEXT NOT NULL,
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);`,
	},
	{
		version: 2,
		name:    "create_moderation_audit_log",
		sql: `
CREATE TABLE IF NOT EXISTS moderation_audit_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id TEXT NOT NULL,
	username TEXT NOT NULL,
	reason TEXT NOT NULL,
	content_preview TEXT NOT NULL,
	content_length INTEGER NOT NULL,
	timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_moderation_audit_log_timestamp
	ON moderation_audit_log(timestamp);`,
	},
	{
		version: 3,
		name:    "index_message_timestamp",
		sql: `
CREATE INDEX IF NOT EXISTS idx_messages_timestamp
	ON messages(julianday(timestamp));`,
	},
	{
		version: 4,
		name:    "index_moderation_audit_timestamp",
		sql: `
CREATE INDEX IF NOT EXISTS idx_moderation_audit_log_julianday_timestamp
		ON moderation_audit_log(julianday(timestamp));`,
	},
	{
		version: 5,
		name:    "add_message_rooms",
		sql: `
ALTER TABLE messages ADD COLUMN room TEXT NOT NULL DEFAULT 'general';
CREATE INDEX IF NOT EXISTS idx_messages_room_timestamp
	ON messages(room, julianday(timestamp));`,
	},
	{
		version: 6,
		name:    "bind_message_and_audit_user_ids",
		sql: `
ALTER TABLE messages ADD COLUMN user_id TEXT NOT NULL DEFAULT '';
ALTER TABLE moderation_audit_log ADD COLUMN user_id TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_messages_user_id_timestamp
	ON messages(user_id, julianday(timestamp)) WHERE user_id <> '';`,
	},
}

// Message represents a chat message
type Message struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id,omitempty"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	Type      string    `json:"type"`
	Room      string    `json:"room"`
	Timestamp time.Time `json:"timestamp"`
}

// ModerationEvent records a rejected chat input for abuse/debug auditing.
type ModerationEvent struct {
	ID             int64     `json:"id"`
	SessionID      string    `json:"session_id"`
	UserID         string    `json:"user_id,omitempty"`
	Username       string    `json:"username"`
	Reason         string    `json:"reason"`
	ContentPreview string    `json:"content_preview"`
	ContentLength  int       `json:"content_length"`
	Timestamp      time.Time `json:"timestamp"`
}

// DB is our database wrapper
type DB struct {
	*sql.DB
}

// New creates a new database connection at the default path (./chatster.db).
func New() (*DB, error) {
	return Open("./chatster.db")
}

// Open creates a database connection at the given SQLite file path.
func Open(path string) (*DB, error) {
	database, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)

	if err := configureSQLite(database); err != nil {
		_ = database.Close()
		return nil, err
	}

	if err := database.Ping(); err != nil {
		_ = database.Close()
		return nil, err
	}

	if err := runMigrations(database); err != nil {
		_ = database.Close()
		return nil, err
	}

	return &DB{database}, nil
}

func configureSQLite(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		fmt.Sprintf("PRAGMA busy_timeout = %d", sqliteBusyTimeoutMS),
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return err
		}
	}
	return nil
}

func runMigrations(db *sql.DB) error {
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
	version INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`); err != nil {
		return err
	}

	for _, m := range migrations {
		applied, err := migrationApplied(db, m.version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := applyMigration(db, m); err != nil {
			return err
		}
	}

	log.Println("Database migrations applied successfully")
	return nil
}

func migrationApplied(db *sql.DB, version int) (bool, error) {
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

func applyMigration(db *sql.DB, m migration) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(m.sql); err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT INTO schema_migrations(version, name) VALUES(?, ?)", m.version, m.name); err != nil {
		return err
	}
	return tx.Commit()
}

// SaveMessage saves a message to the database
func (db *DB) SaveMessage(username, content, msgType string) (*Message, error) {
	return db.SaveMessageInRoom(DefaultRoom, username, content, msgType)
}

// SaveMessageInRoom saves a message to the database for a validated room.
func (db *DB) SaveMessageInRoom(room, username, content, msgType string) (*Message, error) {
	return db.SaveMessageInRoomContext(context.Background(), room, username, content, msgType)
}

// SaveMessageInRoomContext saves a message with a caller-controlled context.
func (db *DB) SaveMessageInRoomContext(ctx context.Context, room, username, content, msgType string) (*Message, error) {
	return db.SaveMessageForUserInRoomContext(ctx, room, "", username, content, msgType)
}

// SaveMessageForUserInRoomContext saves a message with stable identity separate from its display name.
func (db *DB) SaveMessageForUserInRoomContext(ctx context.Context, room, userID, username, content, msgType string) (*Message, error) {
	room, err := NormalizeRoom(room)
	if err != nil {
		return nil, err
	}

	// Don't save system messages
	if msgType == "username" {
		now := time.Now().UTC()
		return &Message{
			UserID:    userID,
			Username:  username,
			Content:   content,
			Type:      msgType,
			Room:      room,
			Timestamp: now,
		}, nil
	}

	stmt, err := db.PrepareContext(ctx, "INSERT INTO messages(room, user_id, username, content, type, timestamp) VALUES(?, ?, ?, ?, ?, ?)")
	if err != nil {
		return nil, err
	}
	defer func() { _ = stmt.Close() }()

	now := time.Now().UTC()
	result, err := stmt.ExecContext(ctx, room, userID, username, content, msgType, now)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Message{
		ID:        id,
		UserID:    userID,
		Username:  username,
		Content:   content,
		Type:      msgType,
		Room:      room,
		Timestamp: now,
	}, nil
}

// PruneMessagesBefore deletes messages older than cutoff and returns the deleted row count.
func (db *DB) PruneMessagesBefore(cutoff time.Time) (int64, error) {
	return db.PruneMessagesBeforeContext(context.Background(), cutoff)
}

// PruneMessagesBeforeContext deletes messages with a caller-controlled context.
func (db *DB) PruneMessagesBeforeContext(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := db.ExecContext(ctx,
		"DELETE FROM messages WHERE julianday(timestamp) < julianday(?)",
		cutoff.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// PruneModerationEventsBefore deletes audit events older than cutoff and returns the deleted row count.
func (db *DB) PruneModerationEventsBefore(cutoff time.Time) (int64, error) {
	return db.PruneModerationEventsBeforeContext(context.Background(), cutoff)
}

// PruneModerationEventsBeforeContext deletes audit events with a caller-controlled context.
func (db *DB) PruneModerationEventsBeforeContext(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := db.ExecContext(ctx,
		"DELETE FROM moderation_audit_log WHERE julianday(timestamp) < julianday(?)",
		cutoff.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// SaveModerationEvent records a rejected message or username attempt.
func (db *DB) SaveModerationEvent(sessionID, username, reason, content string) (*ModerationEvent, error) {
	return db.SaveModerationEventContext(context.Background(), sessionID, username, reason, content)
}

// SaveModerationEventContext records an audit event with a caller-controlled context.
func (db *DB) SaveModerationEventContext(ctx context.Context, sessionID, username, reason, content string) (*ModerationEvent, error) {
	return db.SaveModerationEventForUserContext(ctx, sessionID, "", username, reason, content)
}

// SaveModerationEventForUserContext records a rejected input with its authenticated user when available.
func (db *DB) SaveModerationEventForUserContext(ctx context.Context, sessionID, userID, username, reason, content string) (*ModerationEvent, error) {
	contentPreview := truncateRunes(content, maxAuditPreviewRunes)
	contentLength := utf8.RuneCountInString(content)
	now := time.Now().UTC()

	stmt, err := db.PrepareContext(ctx, `
INSERT INTO moderation_audit_log(
	session_id,
	user_id,
	username,
	reason,
	content_preview,
	content_length,
	timestamp
) VALUES(?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = stmt.Close() }()

	result, err := stmt.ExecContext(ctx, sessionID, userID, username, reason, contentPreview, contentLength, now)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &ModerationEvent{
		ID:             id,
		SessionID:      sessionID,
		UserID:         userID,
		Username:       username,
		Reason:         reason,
		ContentPreview: contentPreview,
		ContentLength:  contentLength,
		Timestamp:      now,
	}, nil
}

// GetRecentMessages retrieves the most recent messages from the database
func (db *DB) GetRecentMessages(limit int) ([]Message, error) {
	return db.GetRecentMessagesInRoom(DefaultRoom, limit)
}

// GetRecentMessagesInRoom retrieves the most recent messages for a validated room.
func (db *DB) GetRecentMessagesInRoom(room string, limit int) ([]Message, error) {
	return db.GetRecentMessagesInRoomContext(context.Background(), room, limit)
}

// GetRecentMessagesInRoomContext retrieves history with a caller-controlled context.
func (db *DB) GetRecentMessagesInRoomContext(ctx context.Context, room string, limit int) ([]Message, error) {
	room, err := NormalizeRoom(room)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, "SELECT id, room, user_id, username, content, type, timestamp FROM messages WHERE room = ? ORDER BY timestamp DESC, id DESC LIMIT ?", room, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var messages []Message
	for rows.Next() {
		var msg Message
		var timestamp string
		if err := rows.Scan(&msg.ID, &msg.Room, &msg.UserID, &msg.Username, &msg.Content, &msg.Type, &timestamp); err != nil {
			return nil, err
		}

		ts, err := parseMsgTimestamp(timestamp)
		if err != nil {
			msg.Timestamp = time.Now().UTC()
		} else {
			msg.Timestamp = ts
		}

		messages = append(messages, msg)
	}

	// Reverse the messages to get them in chronological order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}

// NormalizeRoom returns the canonical room name or an error for invalid input.
func NormalizeRoom(room string) (string, error) {
	room = strings.ToLower(strings.TrimSpace(room))
	if room == "" {
		return DefaultRoom, nil
	}

	runes := []rune(room)
	if len(runes) > maxRoomRunes {
		return "", fmt.Errorf("room must be at most %d characters", maxRoomRunes)
	}

	for i, r := range runes {
		isLetter := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if i == 0 {
			if !isLetter && !isDigit {
				return "", fmt.Errorf("room must start with a letter or digit")
			}
			continue
		}
		if !isLetter && !isDigit && r != '_' && r != '-' {
			return "", fmt.Errorf("room may contain only letters, digits, hyphens, and underscores")
		}
	}

	return room, nil
}

func truncateRunes(s string, limit int) string {
	if limit < 1 || utf8.RuneCountInString(s) <= limit {
		return s
	}

	runes := []rune(s)
	return string(runes[:limit])
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}

func parseMsgTimestamp(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999999-07:00",
	}
	var last error
	for _, layout := range layouts {
		t, err := time.ParseInLocation(layout, s, time.Local)
		if err == nil {
			return t, nil
		}
		last = err
	}
	return time.Time{}, last
}

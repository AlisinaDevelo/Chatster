package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3" // Driver registers with database/sql as "sqlite3".
)

const sqliteBusyTimeoutMS = 5000

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
}

// Message represents a chat message
type Message struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
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
	// Don't save system messages
	if msgType == "username" {
		return &Message{
			Username:  username,
			Content:   content,
			Type:      msgType,
			Timestamp: time.Now(),
		}, nil
	}

	stmt, err := db.Prepare("INSERT INTO messages(username, content, type, timestamp) VALUES(?, ?, ?, ?)")
	if err != nil {
		return nil, err
	}
	defer func() { _ = stmt.Close() }()

	now := time.Now()
	result, err := stmt.Exec(username, content, msgType, now)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return &Message{
		ID:        id,
		Username:  username,
		Content:   content,
		Type:      msgType,
		Timestamp: now,
	}, nil
}

// GetRecentMessages retrieves the most recent messages from the database
func (db *DB) GetRecentMessages(limit int) ([]Message, error) {
	rows, err := db.Query("SELECT id, username, content, type, timestamp FROM messages ORDER BY timestamp DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var messages []Message
	for rows.Next() {
		var msg Message
		var timestamp string
		if err := rows.Scan(&msg.ID, &msg.Username, &msg.Content, &msg.Type, &timestamp); err != nil {
			return nil, err
		}

		ts, err := parseMsgTimestamp(timestamp)
		if err != nil {
			msg.Timestamp = time.Now()
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

package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3" // Driver registers with database/sql as "sqlite3".
)

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

	if err := database.Ping(); err != nil {
		_ = database.Close()
		return nil, err
	}

	if err := initDB(database); err != nil {
		_ = database.Close()
		return nil, err
	}

	return &DB{database}, nil
}

// initDB creates the tables if they don't exist
func initDB(db *sql.DB) error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL,
		content TEXT NOT NULL,
		type TEXT NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	_, err := db.Exec(createTableSQL)
	if err != nil {
		return err
	}

	log.Println("Database initialized successfully")
	return nil
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

		// Parse the timestamp
		t, err := time.Parse("2006-01-02 15:04:05", timestamp)
		if err != nil {
			fmt.Printf("Error parsing time: %v\n", err)
			msg.Timestamp = time.Now()
		} else {
			msg.Timestamp = t
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

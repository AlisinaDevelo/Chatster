package db

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const (
	// DefaultSQLitePath is the SQLite path used when no path is configured.
	DefaultSQLitePath = "./chatster.db"
	// DefaultPostgresMinConns is the default minimum Postgres pool size.
	DefaultPostgresMinConns = int32(2)
	// DefaultPostgresMaxConns is the default maximum Postgres pool size.
	DefaultPostgresMaxConns = int32(10)
)

// Repository is the storage boundary used by the application runtime.
// Concrete backends own their driver, migrations, and connection lifecycle.
type Repository interface {
	SaveMessageInRoomContext(ctx context.Context, room, username, content, msgType string) (*Message, error)
	GetRecentMessagesInRoomContext(ctx context.Context, room string, limit int) ([]Message, error)
	SaveModerationEventContext(ctx context.Context, sessionID, username, reason, content string) (*ModerationEvent, error)
	PruneMessagesBeforeContext(ctx context.Context, cutoff time.Time) (int64, error)
	PruneModerationEventsBeforeContext(ctx context.Context, cutoff time.Time) (int64, error)
	PingContext(ctx context.Context) error
	Close() error
}

// StorageConfig selects exactly one repository backend.
type StorageConfig struct {
	Storage          string
	SQLitePath       string
	PostgresDSN      string
	PostgresMinConns int32
	PostgresMaxConns int32
}

// StartupError identifies the backend operation that prevented startup.
// It intentionally does not include connection strings or secret values.
type StartupError struct {
	Backend   string
	Operation string
	Err       error
}

func (e *StartupError) Error() string {
	return fmt.Sprintf("%s storage %s failed: %v", e.Backend, e.Operation, e.Err)
}

func (e *StartupError) Unwrap() error {
	return e.Err
}

// OpenRepository selects and initializes the configured backend without fallback.
func OpenRepository(ctx context.Context, cfg StorageConfig) (Repository, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	backend := strings.ToLower(strings.TrimSpace(cfg.Storage))
	if backend == "" {
		backend = "sqlite"
	}

	switch backend {
	case "sqlite":
		path := strings.TrimSpace(cfg.SQLitePath)
		if path == "" {
			path = DefaultSQLitePath
		}
		database, err := Open(path)
		if err != nil {
			return nil, &StartupError{Backend: backend, Operation: "open", Err: err}
		}
		return database, nil
	case "postgres":
		minConns := cfg.PostgresMinConns
		maxConns := cfg.PostgresMaxConns
		if minConns == 0 {
			minConns = DefaultPostgresMinConns
		}
		if maxConns == 0 {
			maxConns = DefaultPostgresMaxConns
		}
		postgres, err := OpenPostgres(ctx, cfg.PostgresDSN, minConns, maxConns)
		if err != nil {
			return nil, &StartupError{Backend: backend, Operation: "open", Err: err}
		}
		return postgres, nil
	default:
		return nil, &StartupError{
			Backend:   backend,
			Operation: "select",
			Err:       fmt.Errorf("unsupported backend %q; use sqlite or postgres", backend),
		}
	}
}

var _ Repository = (*DB)(nil)
var _ Repository = (*PostgresDB)(nil)

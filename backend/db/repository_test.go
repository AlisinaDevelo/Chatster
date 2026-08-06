package db

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenRepositoryDefaultsToSQLite(t *testing.T) {
	repository, err := OpenRepository(context.Background(), StorageConfig{
		SQLitePath: filepath.Join(t.TempDir(), "default.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = repository.Close() }()

	if _, ok := repository.(*DB); !ok {
		t.Fatalf("default repository type: %T", repository)
	}
	if _, err := repository.SaveMessageInRoomContext(context.Background(), DefaultRoom, "alice", "hello", "message"); err != nil {
		t.Fatalf("save through repository: %v", err)
	}
}

func TestOpenRepositoryRejectsUnsupportedBackend(t *testing.T) {
	repository, err := OpenRepository(context.Background(), StorageConfig{Storage: "mysql"})
	if repository != nil {
		t.Fatalf("unsupported backend returned repository %T", repository)
	}
	var startupErr *StartupError
	if !errors.As(err, &startupErr) {
		t.Fatalf("error type: %T %v", err, err)
	}
	if startupErr.Backend != "mysql" || startupErr.Operation != "select" {
		t.Fatalf("startup error: %#v", startupErr)
	}
}

func TestOpenRepositoryRequiresPostgresDSN(t *testing.T) {
	repository, err := OpenRepository(context.Background(), StorageConfig{Storage: "postgres"})
	if repository != nil {
		t.Fatalf("missing DSN returned repository %T", repository)
	}
	var startupErr *StartupError
	if !errors.As(err, &startupErr) {
		t.Fatalf("error type: %T %v", err, err)
	}
	if startupErr.Backend != "postgres" || startupErr.Operation != "open" {
		t.Fatalf("startup error: %#v", startupErr)
	}
}

func TestOpenRepositoryRejectsInvalidPostgresPool(t *testing.T) {
	for name, cfg := range map[string]StorageConfig{
		"negative minimum": {Storage: "postgres", PostgresDSN: "postgres://localhost/chatster", PostgresMinConns: -1, PostgresMaxConns: 2},
		"zero maximum":     {Storage: "postgres", PostgresDSN: "postgres://localhost/chatster", PostgresMinConns: 1, PostgresMaxConns: -1},
		"maximum below minimum": {
			Storage: "postgres", PostgresDSN: "postgres://localhost/chatster", PostgresMinConns: 3, PostgresMaxConns: 2,
		},
	} {
		t.Run(name, func(t *testing.T) {
			repository, err := OpenRepository(context.Background(), cfg)
			if repository != nil {
				t.Fatalf("invalid pool returned repository %T", repository)
			}
			if err == nil {
				t.Fatal("expected invalid pool error")
			}
		})
	}
}

func TestOpenPostgresDoesNotEchoInvalidDSN(t *testing.T) {
	secret := "super-secret"
	_, err := OpenPostgres(context.Background(), "postgres://chatster:"+secret+"@%zz", 1, 2)
	if err == nil {
		t.Fatal("expected invalid DSN error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("invalid DSN error echoed a secret: %v", err)
	}
}

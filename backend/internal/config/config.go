package config

import (
	"os"
	"strings"
)

const (
	defaultHTTPAddr = ":8080"
	defaultDBPath   = "./chatster.db"
)

// Config holds process configuration loaded from the environment.
type Config struct {
	HTTPAddr string
	DBPath   string
}

// FromEnv reads configuration from environment variables with safe defaults.
//
// CHATSTER_HTTP_ADDR — listen address (default ":8080").
// CHATSTER_DB_PATH   — SQLite file path (default "./chatster.db").
func FromEnv() Config {
	cfg := Config{
		HTTPAddr: strings.TrimSpace(os.Getenv("CHATSTER_HTTP_ADDR")),
		DBPath:   strings.TrimSpace(os.Getenv("CHATSTER_DB_PATH")),
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = defaultHTTPAddr
	}
	if cfg.DBPath == "" {
		cfg.DBPath = defaultDBPath
	}
	return cfg
}

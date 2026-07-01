package config

import (
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	defaultHTTPAddr       = ":8080"
	defaultDBPath         = "./chatster.db"
	defaultWSUpgradeRPS   = 5.0
	defaultWSUpgradeBurst = 10
	defaultMessageRPS     = 5.0
	defaultMessageBurst   = 10
)

// Config holds process configuration loaded from the environment.
type Config struct {
	HTTPAddr                string
	DBPath                  string
	StaticDir               string
	AllowedOrigins          []string
	WSUpgradeRPS            float64
	WSUpgradeBurst          int
	DisableWSRateLimit      bool
	MessageRPS              float64
	MessageBurst            int
	DisableMessageRateLimit bool
}

// FromEnv reads configuration from environment variables with safe defaults.
//
// CHATSTER_HTTP_ADDR — listen address (default ":8080").
// CHATSTER_DB_PATH — SQLite file path (default "./chatster.db").
// CHATSTER_STATIC_DIR — optional directory of built frontend assets to serve from the backend.
// CHATSTER_ALLOWED_ORIGINS — comma-separated WebSocket Origin allowlist; empty = allow all (dev-friendly).
// CHATSTER_WS_UPGRADE_RPS — max WS upgrades per IP per second (default 5); "0" disables limiting.
// CHATSTER_WS_UPGRADE_BURST — token bucket burst for WS upgrades (default 10).
// CHATSTER_MESSAGE_RPS — max chat messages per client per second (default 5); "0" disables limiting.
// CHATSTER_MESSAGE_BURST — token bucket burst for chat messages (default 10).
func FromEnv() Config {
	cfg := Config{
		HTTPAddr:       strings.TrimSpace(os.Getenv("CHATSTER_HTTP_ADDR")),
		DBPath:         strings.TrimSpace(os.Getenv("CHATSTER_DB_PATH")),
		StaticDir:      strings.TrimSpace(os.Getenv("CHATSTER_STATIC_DIR")),
		AllowedOrigins: splitCSV(os.Getenv("CHATSTER_ALLOWED_ORIGINS")),
		WSUpgradeRPS:   defaultWSUpgradeRPS,
		WSUpgradeBurst: defaultWSUpgradeBurst,
		MessageRPS:     defaultMessageRPS,
		MessageBurst:   defaultMessageBurst,
	}

	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = defaultHTTPAddr
	}
	if cfg.DBPath == "" {
		cfg.DBPath = defaultDBPath
	}

	switch v := strings.TrimSpace(os.Getenv("CHATSTER_WS_UPGRADE_RPS")); v {
	case "0":
		cfg.DisableWSRateLimit = true
	case "":
		// defaults
	default:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			if f <= 0 {
				cfg.DisableWSRateLimit = true
			} else {
				cfg.WSUpgradeRPS = f
			}
		}
	}

	if v := strings.TrimSpace(os.Getenv("CHATSTER_WS_UPGRADE_BURST")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.WSUpgradeBurst = n
		}
	}

	switch v := strings.TrimSpace(os.Getenv("CHATSTER_MESSAGE_RPS")); v {
	case "0":
		cfg.DisableMessageRateLimit = true
	case "":
		// defaults
	default:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			if f <= 0 {
				cfg.DisableMessageRateLimit = true
			} else {
				cfg.MessageRPS = f
			}
		}
	}

	if v := strings.TrimSpace(os.Getenv("CHATSTER_MESSAGE_BURST")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MessageBurst = n
		}
	}

	return cfg
}

// OriginAllowed returns true if the request may open a WebSocket from a browser Origin.
// Empty AllowedOrigins allows any origin (demo default). Missing Origin header is allowed (non-browser clients).
func (c Config) OriginAllowed(r *http.Request) bool {
	if len(c.AllowedOrigins) == 0 {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	for _, o := range c.AllowedOrigins {
		if o == origin {
			return true
		}
	}
	return false
}

// splitCSV splits a comma-separated list into trimmed non-empty tokens.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

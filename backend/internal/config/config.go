package config

import (
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	defaultHTTPAddr             = ":8080"
	defaultDBPath               = "./chatster.db"
	defaultStorage              = "sqlite"
	defaultPostgresMinConns     = int32(2)
	defaultPostgresMaxConns     = int32(10)
	defaultWSUpgradeRPS         = 5.0
	defaultWSUpgradeBurst       = 10
	defaultMessageRPS           = 5.0
	defaultMessageBurst         = 10
	defaultMessageRetentionDays = 0
	defaultAuditRetentionDays   = 0
	defaultRedisNamespace       = "development"
)

// Config holds process configuration loaded from the environment.
type Config struct {
	HTTPAddr                string
	DBPath                  string
	Storage                 string
	PostgresDSN             string
	PostgresMinConns        int32
	PostgresMaxConns        int32
	RedisURL                string
	RedisNamespace          string
	InstanceID              string
	StaticDir               string
	AllowedOrigins          []string
	WSUpgradeRPS            float64
	WSUpgradeBurst          int
	DisableWSRateLimit      bool
	MessageRPS              float64
	MessageBurst            int
	DisableMessageRateLimit bool
	MessageRetentionDays    int
	AuditRetentionDays      int
}

// FromEnv reads configuration from environment variables with safe defaults.
//
// CHATSTER_HTTP_ADDR — listen address; if unset, use numeric PORT when available, otherwise ":8080".
// CHATSTER_DB_PATH — SQLite file path (default "./chatster.db").
// CHATSTER_STORAGE — storage backend: "sqlite" (default) or "postgres"; unknown values fail startup.
// CHATSTER_POSTGRES_DSN — Postgres connection string, required when storage is "postgres"; never logged.
// CHATSTER_POSTGRES_MIN_CONNS — Postgres pool minimum (default 2); invalid values fail startup in Postgres mode.
// CHATSTER_POSTGRES_MAX_CONNS — Postgres pool maximum (default 10); invalid values fail startup in Postgres mode.
// CHATSTER_REDIS_URL — optional Redis URL; setting it enables cross-instance Pub/Sub fan-out and validates connectivity at startup.
// CHATSTER_REDIS_NAMESPACE — environment namespace for Redis channels (default "development").
// CHATSTER_INSTANCE_ID — optional process identifier for loop prevention; generated when Redis is enabled and unset.
// CHATSTER_STATIC_DIR — optional directory of built frontend assets to serve from the backend.
// CHATSTER_ALLOWED_ORIGINS — comma-separated WebSocket Origin allowlist; empty = allow all (dev-friendly).
// CHATSTER_WS_UPGRADE_RPS — max WS upgrades per IP per second (default 5); "0" disables limiting.
// CHATSTER_WS_UPGRADE_BURST — token bucket burst for WS upgrades (default 10).
// CHATSTER_MESSAGE_RPS — max chat messages per client per second (default 5); "0" disables limiting.
// CHATSTER_MESSAGE_BURST — token bucket burst for chat messages (default 10).
// CHATSTER_MESSAGE_RETENTION_DAYS — delete messages older than this many days at startup (default 0 = disabled).
// CHATSTER_AUDIT_RETENTION_DAYS — delete moderation audit events older than this many days at startup (default 0 = disabled).
func FromEnv() Config {
	cfg := Config{
		HTTPAddr:         strings.TrimSpace(os.Getenv("CHATSTER_HTTP_ADDR")),
		DBPath:           strings.TrimSpace(os.Getenv("CHATSTER_DB_PATH")),
		Storage:          strings.ToLower(strings.TrimSpace(os.Getenv("CHATSTER_STORAGE"))),
		PostgresDSN:      strings.TrimSpace(os.Getenv("CHATSTER_POSTGRES_DSN")),
		PostgresMinConns: parsePoolSizeEnv("CHATSTER_POSTGRES_MIN_CONNS", defaultPostgresMinConns),
		PostgresMaxConns: parsePoolSizeEnv("CHATSTER_POSTGRES_MAX_CONNS", defaultPostgresMaxConns),
		RedisURL:         strings.TrimSpace(os.Getenv("CHATSTER_REDIS_URL")),
		RedisNamespace:   strings.TrimSpace(os.Getenv("CHATSTER_REDIS_NAMESPACE")),
		InstanceID:       strings.TrimSpace(os.Getenv("CHATSTER_INSTANCE_ID")),
		StaticDir:        strings.TrimSpace(os.Getenv("CHATSTER_STATIC_DIR")),
		AllowedOrigins:   splitCSV(os.Getenv("CHATSTER_ALLOWED_ORIGINS")),
		WSUpgradeRPS:     defaultWSUpgradeRPS,
		WSUpgradeBurst:   defaultWSUpgradeBurst,
		MessageRPS:       defaultMessageRPS,
		MessageBurst:     defaultMessageBurst,
	}

	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = platformHTTPAddr()
	}
	if cfg.DBPath == "" {
		cfg.DBPath = defaultDBPath
	}
	if cfg.Storage == "" {
		cfg.Storage = defaultStorage
	}
	if cfg.RedisNamespace == "" {
		cfg.RedisNamespace = defaultRedisNamespace
	}

	if v := strings.TrimSpace(os.Getenv("CHATSTER_MESSAGE_RETENTION_DAYS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MessageRetentionDays = n
		}
	}

	if v := strings.TrimSpace(os.Getenv("CHATSTER_AUDIT_RETENTION_DAYS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.AuditRetentionDays = n
		}
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

// parsePoolSizeEnv preserves an invalid explicit value as -1 so Postgres mode can fail fast.
func parsePoolSizeEnv(name string, defaultValue int32) int32 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return defaultValue
	}

	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || n < 1 {
		return -1
	}
	return int32(n)
}

// platformHTTPAddr adapts the numeric PORT convention used by hosted web services.
func platformHTTPAddr() string {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		return defaultHTTPAddr
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return defaultHTTPAddr
	}
	return ":" + strconv.Itoa(portNumber)
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

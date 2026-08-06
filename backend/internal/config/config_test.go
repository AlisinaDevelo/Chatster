package config

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFromEnvDefaults(t *testing.T) {
	t.Setenv("CHATSTER_HTTP_ADDR", "")
	t.Setenv("PORT", "")
	t.Setenv("CHATSTER_DB_PATH", "")
	t.Setenv("CHATSTER_STORAGE", "")
	t.Setenv("CHATSTER_POSTGRES_DSN", "")
	t.Setenv("CHATSTER_POSTGRES_MIN_CONNS", "")
	t.Setenv("CHATSTER_POSTGRES_MAX_CONNS", "")
	t.Setenv("CHATSTER_STATIC_DIR", "")
	t.Setenv("CHATSTER_ALLOWED_ORIGINS", "")
	t.Setenv("CHATSTER_MESSAGE_RETENTION_DAYS", "")
	t.Setenv("CHATSTER_AUDIT_RETENTION_DAYS", "")
	t.Setenv("CHATSTER_WS_UPGRADE_RPS", "")
	t.Setenv("CHATSTER_WS_UPGRADE_BURST", "")
	cfg := FromEnv()
	if cfg.HTTPAddr != defaultHTTPAddr {
		t.Fatalf("HTTPAddr: got %q want %q", cfg.HTTPAddr, defaultHTTPAddr)
	}
	if cfg.DBPath != defaultDBPath {
		t.Fatalf("DBPath: got %q want %q", cfg.DBPath, defaultDBPath)
	}
	if cfg.Storage != defaultStorage {
		t.Fatalf("Storage: got %q want %q", cfg.Storage, defaultStorage)
	}
	if cfg.PostgresMinConns != defaultPostgresMinConns || cfg.PostgresMaxConns != defaultPostgresMaxConns {
		t.Fatalf("Postgres pool defaults: min=%d max=%d", cfg.PostgresMinConns, cfg.PostgresMaxConns)
	}
	if cfg.StaticDir != "" {
		t.Fatalf("StaticDir: got %q want empty", cfg.StaticDir)
	}
	if cfg.DisableWSRateLimit {
		t.Fatal("expected WS rate limit enabled by default")
	}
	if cfg.WSUpgradeRPS != defaultWSUpgradeRPS {
		t.Fatalf("WSUpgradeRPS: got %v want %v", cfg.WSUpgradeRPS, defaultWSUpgradeRPS)
	}
	if cfg.DisableMessageRateLimit {
		t.Fatal("expected message rate limit enabled by default")
	}
	if cfg.MessageRPS != defaultMessageRPS {
		t.Fatalf("MessageRPS: got %v want %v", cfg.MessageRPS, defaultMessageRPS)
	}
	if cfg.MessageRetentionDays != defaultMessageRetentionDays {
		t.Fatalf("MessageRetentionDays: got %d want %d", cfg.MessageRetentionDays, defaultMessageRetentionDays)
	}
	if cfg.AuditRetentionDays != defaultAuditRetentionDays {
		t.Fatalf("AuditRetentionDays: got %d want %d", cfg.AuditRetentionDays, defaultAuditRetentionDays)
	}
}

func TestFromEnvUsesPlatformPort(t *testing.T) {
	t.Setenv("CHATSTER_HTTP_ADDR", "")
	t.Setenv("PORT", "10000")
	cfg := FromEnv()
	if cfg.HTTPAddr != ":10000" {
		t.Fatalf("HTTPAddr: got %q want :10000", cfg.HTTPAddr)
	}
}

func TestFromEnvIgnoresInvalidPlatformPort(t *testing.T) {
	t.Setenv("CHATSTER_HTTP_ADDR", "")
	t.Setenv("PORT", "not-a-port")
	cfg := FromEnv()
	if cfg.HTTPAddr != defaultHTTPAddr {
		t.Fatalf("HTTPAddr: got %q want %q", cfg.HTTPAddr, defaultHTTPAddr)
	}
}

func TestFromEnvOverride(t *testing.T) {
	t.Setenv("CHATSTER_HTTP_ADDR", ":9999")
	t.Setenv("PORT", "10000")
	t.Setenv("CHATSTER_DB_PATH", "/tmp/x.db")
	t.Setenv("CHATSTER_STORAGE", " POSTGRES ")
	t.Setenv("CHATSTER_POSTGRES_DSN", " postgres://chatster:secret@db/chatster ")
	t.Setenv("CHATSTER_POSTGRES_MIN_CONNS", "3")
	t.Setenv("CHATSTER_POSTGRES_MAX_CONNS", "12")
	t.Setenv("CHATSTER_STATIC_DIR", "/app/static")
	t.Setenv("CHATSTER_ALLOWED_ORIGINS", " https://a.test , https://b.test ")
	t.Setenv("CHATSTER_MESSAGE_RETENTION_DAYS", "30")
	t.Setenv("CHATSTER_AUDIT_RETENTION_DAYS", "90")
	t.Setenv("CHATSTER_WS_UPGRADE_RPS", "12")
	t.Setenv("CHATSTER_WS_UPGRADE_BURST", "3")
	t.Setenv("CHATSTER_MESSAGE_RPS", "8")
	t.Setenv("CHATSTER_MESSAGE_BURST", "4")
	cfg := FromEnv()
	if cfg.HTTPAddr != ":9999" || cfg.DBPath != "/tmp/x.db" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
	if cfg.Storage != "postgres" || cfg.PostgresDSN != "postgres://chatster:secret@db/chatster" {
		t.Fatalf("storage config: %+v", cfg)
	}
	if cfg.PostgresMinConns != 3 || cfg.PostgresMaxConns != 12 {
		t.Fatalf("Postgres pool: min=%d max=%d", cfg.PostgresMinConns, cfg.PostgresMaxConns)
	}
	if cfg.StaticDir != "/app/static" {
		t.Fatalf("StaticDir: got %q want /app/static", cfg.StaticDir)
	}
	if cfg.MessageRetentionDays != 30 {
		t.Fatalf("MessageRetentionDays: got %d want 30", cfg.MessageRetentionDays)
	}
	if cfg.AuditRetentionDays != 90 {
		t.Fatalf("AuditRetentionDays: got %d want 90", cfg.AuditRetentionDays)
	}
	if len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[0] != "https://a.test" {
		t.Fatalf("AllowedOrigins: %+v", cfg.AllowedOrigins)
	}
	if cfg.WSUpgradeRPS != 12 || cfg.WSUpgradeBurst != 3 {
		t.Fatalf("rate: rps=%v burst=%v", cfg.WSUpgradeRPS, cfg.WSUpgradeBurst)
	}
	if cfg.MessageRPS != 8 || cfg.MessageBurst != 4 {
		t.Fatalf("message rate: rps=%v burst=%v", cfg.MessageRPS, cfg.MessageBurst)
	}
}

func TestFromEnvPreservesInvalidPostgresPoolValues(t *testing.T) {
	t.Setenv("CHATSTER_POSTGRES_MIN_CONNS", "0")
	cfg := FromEnv()
	if cfg.PostgresMinConns != -1 {
		t.Fatalf("invalid minimum: got %d want -1", cfg.PostgresMinConns)
	}

	t.Setenv("CHATSTER_POSTGRES_MAX_CONNS", "not-a-number")
	cfg = FromEnv()
	if cfg.PostgresMaxConns != -1 {
		t.Fatalf("invalid maximum: got %d want -1", cfg.PostgresMaxConns)
	}

	t.Setenv("CHATSTER_POSTGRES_MAX_CONNS", "0")
	cfg = FromEnv()
	if cfg.PostgresMaxConns != -1 {
		t.Fatalf("zero maximum: got %d want -1", cfg.PostgresMaxConns)
	}
}

func TestFromEnvIgnoresInvalidMessageRetention(t *testing.T) {
	for _, value := range []string{"0", "-1", "not-a-number"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("CHATSTER_MESSAGE_RETENTION_DAYS", value)
			cfg := FromEnv()
			if cfg.MessageRetentionDays != defaultMessageRetentionDays {
				t.Fatalf("MessageRetentionDays: got %d want %d", cfg.MessageRetentionDays, defaultMessageRetentionDays)
			}
		})
	}
}

func TestFromEnvIgnoresInvalidAuditRetention(t *testing.T) {
	for _, value := range []string{"0", "-1", "not-a-number"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("CHATSTER_AUDIT_RETENTION_DAYS", value)
			cfg := FromEnv()
			if cfg.AuditRetentionDays != defaultAuditRetentionDays {
				t.Fatalf("AuditRetentionDays: got %d want %d", cfg.AuditRetentionDays, defaultAuditRetentionDays)
			}
		})
	}
}

func TestFromEnvWSRateDisabled(t *testing.T) {
	t.Setenv("CHATSTER_WS_UPGRADE_RPS", "0")
	cfg := FromEnv()
	if !cfg.DisableWSRateLimit {
		t.Fatal("want rate limit disabled")
	}
}

func TestFromEnvMessageRateDisabled(t *testing.T) {
	t.Setenv("CHATSTER_MESSAGE_RPS", "0")
	cfg := FromEnv()
	if !cfg.DisableMessageRateLimit {
		t.Fatal("want message rate limit disabled")
	}
}

func TestOriginAllowed(t *testing.T) {
	cfg := Config{AllowedOrigins: []string{"https://app.example"}}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Origin", "https://evil.test")
	if cfg.OriginAllowed(r) {
		t.Fatal("evil origin should be denied")
	}
	r.Header.Set("Origin", "https://app.example")
	if !cfg.OriginAllowed(r) {
		t.Fatal("allowed origin should pass")
	}
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	if !cfg.OriginAllowed(r2) {
		t.Fatal("missing Origin should pass for non-browser clients")
	}
}

package config

import (
	"testing"
)

func TestFromEnvDefaults(t *testing.T) {
	t.Setenv("CHATSTER_HTTP_ADDR", "")
	t.Setenv("CHATSTER_DB_PATH", "")
	cfg := FromEnv()
	if cfg.HTTPAddr != defaultHTTPAddr {
		t.Fatalf("HTTPAddr: got %q want %q", cfg.HTTPAddr, defaultHTTPAddr)
	}
	if cfg.DBPath != defaultDBPath {
		t.Fatalf("DBPath: got %q want %q", cfg.DBPath, defaultDBPath)
	}
}

func TestFromEnvOverride(t *testing.T) {
	t.Setenv("CHATSTER_HTTP_ADDR", ":9999")
	t.Setenv("CHATSTER_DB_PATH", "/tmp/x.db")
	cfg := FromEnv()
	if cfg.HTTPAddr != ":9999" || cfg.DBPath != "/tmp/x.db" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

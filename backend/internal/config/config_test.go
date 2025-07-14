package config

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFromEnvDefaults(t *testing.T) {
	t.Setenv("CHATSTER_HTTP_ADDR", "")
	t.Setenv("CHATSTER_DB_PATH", "")
	t.Setenv("CHATSTER_ALLOWED_ORIGINS", "")
	t.Setenv("CHATSTER_WS_UPGRADE_RPS", "")
	t.Setenv("CHATSTER_WS_UPGRADE_BURST", "")
	cfg := FromEnv()
	if cfg.HTTPAddr != defaultHTTPAddr {
		t.Fatalf("HTTPAddr: got %q want %q", cfg.HTTPAddr, defaultHTTPAddr)
	}
	if cfg.DBPath != defaultDBPath {
		t.Fatalf("DBPath: got %q want %q", cfg.DBPath, defaultDBPath)
	}
	if cfg.DisableWSRateLimit {
		t.Fatal("expected WS rate limit enabled by default")
	}
	if cfg.WSUpgradeRPS != defaultWSUpgradeRPS {
		t.Fatalf("WSUpgradeRPS: got %v want %v", cfg.WSUpgradeRPS, defaultWSUpgradeRPS)
	}
}

func TestFromEnvOverride(t *testing.T) {
	t.Setenv("CHATSTER_HTTP_ADDR", ":9999")
	t.Setenv("CHATSTER_DB_PATH", "/tmp/x.db")
	t.Setenv("CHATSTER_ALLOWED_ORIGINS", " https://a.test , https://b.test ")
	t.Setenv("CHATSTER_WS_UPGRADE_RPS", "12")
	t.Setenv("CHATSTER_WS_UPGRADE_BURST", "3")
	cfg := FromEnv()
	if cfg.HTTPAddr != ":9999" || cfg.DBPath != "/tmp/x.db" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
	if len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[0] != "https://a.test" {
		t.Fatalf("AllowedOrigins: %+v", cfg.AllowedOrigins)
	}
	if cfg.WSUpgradeRPS != 12 || cfg.WSUpgradeBurst != 3 {
		t.Fatalf("rate: rps=%v burst=%v", cfg.WSUpgradeRPS, cfg.WSUpgradeBurst)
	}
}

func TestFromEnvWSRateDisabled(t *testing.T) {
	t.Setenv("CHATSTER_WS_UPGRADE_RPS", "0")
	cfg := FromEnv()
	if !cfg.DisableWSRateLimit {
		t.Fatal("want rate limit disabled")
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

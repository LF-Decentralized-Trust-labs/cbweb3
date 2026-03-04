// This file tests environment parsing and default configuration behavior.
package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Setenv("APP_PORT", "9090")
	t.Setenv("AUTH_MODE", "hybrid")
	t.Setenv("AUTH_TOKEN_TTL_SEC", "600")
	t.Setenv("AUTH_MOCK_CLIENTS", "a:s,b:t")
	t.Setenv("POSTGRES_DSN", "postgres://example")

	cfg := Load()
	if cfg.AppPort != "9090" {
		t.Fatalf("unexpected app port: %s", cfg.AppPort)
	}
	if cfg.AuthMode != "hybrid" {
		t.Fatalf("unexpected auth mode: %s", cfg.AuthMode)
	}
	if len(cfg.MockClients) != 2 {
		t.Fatalf("unexpected clients length: %d", len(cfg.MockClients))
	}
	if cfg.PostgresDSN == "" {
		t.Fatal("expected postgres dsn")
	}
}

func TestParseClientSecrets(t *testing.T) {
	t.Parallel()

	m := parseClientSecrets("bank-a:secret-a,invalid,bank-b:secret-b")
	if len(m) != 2 {
		t.Fatalf("expected 2 valid clients, got %d", len(m))
	}
	if m["bank-a"] != "secret-a" {
		t.Fatalf("unexpected secret for bank-a: %s", m["bank-a"])
	}
}

func TestGetEnvIntFallback(t *testing.T) {
	t.Parallel()

	const key = "MISSING_INT_VALUE"
	_ = os.Unsetenv(key)
	if got := getEnvInt(key, 15); got != 15 {
		t.Fatalf("expected fallback 15, got %d", got)
	}
}


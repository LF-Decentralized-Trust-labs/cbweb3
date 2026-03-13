// This file tests environment parsing and default configuration behavior.
package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Setenv("APP_PORT", "9090")
	t.Setenv("IDENTITY_GRPC_ADDR", "localhost:19091")

	cfg := Load()
	if cfg.AppPort != "9090" {
		t.Fatalf("unexpected app port: %s", cfg.AppPort)
	}
	if cfg.IdentityGRPCAddr != "localhost:19091" {
		t.Fatalf("unexpected grpc addr: %s", cfg.IdentityGRPCAddr)
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

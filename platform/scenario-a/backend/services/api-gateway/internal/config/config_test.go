// This file tests environment parsing and default configuration behavior.
package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Setenv("APP_PORT", "9090")
	t.Setenv("AUTH_GRPC_ADDR", "localhost:19091")

	cfg := Load()
	if cfg.AppPort != "9090" {
		t.Fatalf("unexpected app port: %s", cfg.AppPort)
	}
	if cfg.AuthGRPCAddr != "localhost:19091" {
		t.Fatalf("unexpected grpc addr: %s", cfg.AuthGRPCAddr)
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

func TestLoadFiatSymbol(t *testing.T) {
	t.Setenv("FIAT_SYMBOL", "BRL")

	cfg := Load()
	if cfg.FiatSymbol != "BRL" {
		t.Fatalf("expected FiatSymbol=BRL, got %q", cfg.FiatSymbol)
	}
}

func TestLoadFiatSymbol_DefaultEmpty(t *testing.T) {
	t.Parallel()

	_ = os.Unsetenv("FIAT_SYMBOL")
	cfg := Load()
	if cfg.FiatSymbol != "" {
		t.Fatalf("expected empty FiatSymbol when env not set, got %q", cfg.FiatSymbol)
	}
}

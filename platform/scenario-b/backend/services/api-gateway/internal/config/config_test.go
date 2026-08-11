// SPDX-License-Identifier: Apache-2.0

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

// RelayKeyID identifies the entity in service-to-service authentication, and it must be UNIQUE
// across the deployment — the receiver pins one public key per id and the id is what attributes an
// act to a specific sovereign.
//
// BANK_CODE cannot serve as that id: the compose template sets it to the entity's ROLE, so every
// central bank in the topology is "central-bank". Two CBs sharing an id means the registry can pin
// only one of their keys (a map, one entry per id), and "signed by central-bank" would not say WHICH
// central bank — rebuilding the shared-secret problem in asymmetric clothing.
//
// It defaults to BANK_CODE because a commercial bank's code is already unique, and that keeps every
// existing bank deployment working without a new variable.
func TestRelayKeyID(t *testing.T) {
	t.Run("defaults to BANK_CODE", func(t *testing.T) {
		t.Setenv("BANK_CODE", "bank-itau")
		t.Setenv("RELAY_KEY_ID", "")
		if got := Load().RelayKeyID; got != "bank-itau" {
			t.Fatalf("RelayKeyID = %q, want bank-itau", got)
		}
	})
	t.Run("explicit value wins", func(t *testing.T) {
		t.Setenv("BANK_CODE", "central-bank")
		t.Setenv("RELAY_KEY_ID", "central-bank-brazil")
		cfg := Load()
		if cfg.RelayKeyID != "central-bank-brazil" {
			t.Fatalf("RelayKeyID = %q, want central-bank-brazil", cfg.RelayKeyID)
		}
		// BANK_CODE must be untouched: it flows into owner_bank_id on bridge positions and into
		// the reconciliation's self-exclusion, so changing it would need a data migration.
		if cfg.BankCode != "central-bank" {
			t.Fatalf("BankCode = %q, want central-bank — the relay id must not redefine it", cfg.BankCode)
		}
	})
}

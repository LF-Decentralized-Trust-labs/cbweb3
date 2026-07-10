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

func TestGetEnvInt_InvalidFallsBackToDefault(t *testing.T) {
	t.Setenv("BAD_INT_VALUE", "not-a-number")
	if got := getEnvInt("BAD_INT_VALUE", 7); got != 7 {
		t.Fatalf("expected fallback 7 for unparseable value, got %d", got)
	}
}

func TestGetEnvInt_ValidOverride(t *testing.T) {
	t.Setenv("GOOD_INT_VALUE", "42")
	if got := getEnvInt("GOOD_INT_VALUE", 7); got != 42 {
		t.Fatalf("expected parsed value 42, got %d", got)
	}
}

func TestGetEnvBool(t *testing.T) {
	cases := []struct {
		name     string
		set      bool
		value    string
		fallback bool
		want     bool
	}{
		{name: "unset returns fallback true", set: false, fallback: true, want: true},
		{name: "unset returns fallback false", set: false, fallback: false, want: false},
		{name: "true literal", set: true, value: "true", fallback: false, want: true},
		{name: "one literal", set: true, value: "1", fallback: false, want: true},
		{name: "false literal", set: true, value: "false", fallback: true, want: false},
		{name: "unrecognised value is false", set: true, value: "yes", fallback: true, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const key = "SOME_BOOL_VALUE"
			if tc.set {
				t.Setenv(key, tc.value)
			} else {
				_ = os.Unsetenv(key)
			}
			if got := getEnvBool(key, tc.fallback); got != tc.want {
				t.Fatalf("getEnvBool(%q, %v) = %v, want %v", tc.value, tc.fallback, got, tc.want)
			}
		})
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

func TestLoadPaladinIdentities_Default(t *testing.T) {
	_ = os.Unsetenv("PALADIN_IDENTITIES")
	cfg := Load()
	if len(cfg.PaladinIdentities) != len(defaultPaladinIdentities) {
		t.Fatalf("expected %d default identities, got %d", len(defaultPaladinIdentities), len(cfg.PaladinIdentities))
	}
	// Guard against the default drifting back to placeholder spoke-a/spoke-b data.
	if cfg.PaladinIdentities[0] != "funded_operator@spoke-brl-cb" {
		t.Errorf("first default identity = %q, want funded_operator@spoke-brl-cb", cfg.PaladinIdentities[0])
	}
}

func TestLoadPaladinIdentities_Override(t *testing.T) {
	t.Setenv("PALADIN_IDENTITIES", " funded_operator@spoke-a-cb , ,funded_operator@spoke-b-bank-b ")

	cfg := Load()
	want := []string{"funded_operator@spoke-a-cb", "funded_operator@spoke-b-bank-b"}
	if len(cfg.PaladinIdentities) != len(want) {
		t.Fatalf("identities = %v, want %v", cfg.PaladinIdentities, want)
	}
	for i, id := range want {
		if cfg.PaladinIdentities[i] != id {
			t.Errorf("identities[%d] = %q, want %q (blank entries must be trimmed)", i, cfg.PaladinIdentities[i], id)
		}
	}
}

func TestGetEnvList_BlankFallsBackToDefault(t *testing.T) {
	t.Setenv("SOME_LIST", "  , ,  ")

	fallback := []string{"a", "b"}
	got := getEnvList("SOME_LIST", fallback)
	if len(got) != len(fallback) {
		t.Fatalf("expected fallback when all entries blank, got %v", got)
	}
}

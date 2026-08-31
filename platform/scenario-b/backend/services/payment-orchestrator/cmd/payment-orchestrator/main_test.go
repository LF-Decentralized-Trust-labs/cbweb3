// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
)

// TestResolveHubChainID verifies that HUB_CHAIN_ID resolution defaults to 1337,
// not 1338, and emits a structured warning when the variable is unset (FR-007, FR-009).
//
// Constitution Principle V: this test MUST fail (red) before the implementation
// in T016 is written. The function resolveHubChainID does not yet exist.
func TestResolveHubChainID_DefaultsTo1337(t *testing.T) {
	t.Setenv("HUB_CHAIN_ID", "") // explicitly unset

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	got := resolveHubChainID(logger)

	if got != 1337 {
		t.Errorf("resolveHubChainID() = %d; want 1337 (HUB_CHAIN_ID unset must default to 1337, not 1338)", got)
	}
}

func TestResolveHubChainID_EmitsWarningWhenUnset(t *testing.T) {
	os.Unsetenv("HUB_CHAIN_ID")

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	resolveHubChainID(logger)

	if buf.Len() == 0 {
		t.Fatal("expected a warning log when HUB_CHAIN_ID is unset; got none")
	}

	var record map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("warning log is not valid JSON: %v\nraw: %s", err, buf.String())
	}

	if record["level"] != "WARN" {
		t.Errorf("log level = %q; want %q", record["level"], "WARN")
	}
	if msg, _ := record["msg"].(string); msg == "" {
		t.Error("warning log has no msg field")
	}
}

func TestResolveHubChainID_UsesEnvWhenSet(t *testing.T) {
	t.Setenv("HUB_CHAIN_ID", "9999")

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	got := resolveHubChainID(logger)

	if got != 9999 {
		t.Errorf("resolveHubChainID() = %d; want 9999 (respects env override)", got)
	}
	if buf.Len() != 0 {
		t.Error("expected no warning when HUB_CHAIN_ID is explicitly set")
	}
}

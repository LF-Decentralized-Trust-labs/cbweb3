// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"log"
	"os"
	"testing"
)

// TestBootstrapResolveHubChainID_DefaultsTo1337 asserts that identity_bootstrap.go
// resolves the Hub chain ID from configuration (not hardcoded int64(1338)), defaults
// to 1337, and warns when HUB_CHAIN_ID is absent (FR-007, FR-008, FR-009).
//
// Constitution Principle V: this test MUST fail (red) before T018 is written.
// The function resolveBootstrapHubChainID does not yet exist.
func TestBootstrapResolveHubChainID_DefaultsTo1337(t *testing.T) {
	os.Unsetenv("HUB_CHAIN_ID")

	var buf bytes.Buffer
	warnLogger := log.New(&buf, "", 0)

	got := resolveBootstrapHubChainID(warnLogger)

	if got != 1337 {
		t.Errorf("resolveBootstrapHubChainID() = %d; want 1337 (must not hardcode 1338)", got)
	}
}

func TestBootstrapResolveHubChainID_EmitsWarningWhenUnset(t *testing.T) {
	os.Unsetenv("HUB_CHAIN_ID")

	var buf bytes.Buffer
	warnLogger := log.New(&buf, "", 0)

	resolveBootstrapHubChainID(warnLogger)

	if buf.Len() == 0 {
		t.Fatal("expected a warning log when HUB_CHAIN_ID is unset; got none (FR-009)")
	}
}

func TestBootstrapResolveHubChainID_NeverReturns1338(t *testing.T) {
	os.Unsetenv("HUB_CHAIN_ID")

	var buf bytes.Buffer
	warnLogger := log.New(&buf, "", 0)

	got := resolveBootstrapHubChainID(warnLogger)

	if got == 1338 {
		t.Errorf("resolveBootstrapHubChainID() = %d; must NEVER default to 1338 (FR-008)", got)
	}
}

func TestBootstrapResolveHubChainID_UsesEnvWhenSet(t *testing.T) {
	t.Setenv("HUB_CHAIN_ID", "9999")

	var buf bytes.Buffer
	warnLogger := log.New(&buf, "", 0)

	got := resolveBootstrapHubChainID(warnLogger)

	if got != 9999 {
		t.Errorf("resolveBootstrapHubChainID() = %d; want 9999 (must respect env override)", got)
	}
	if buf.Len() != 0 {
		t.Error("expected no warning when HUB_CHAIN_ID is explicitly set")
	}
}

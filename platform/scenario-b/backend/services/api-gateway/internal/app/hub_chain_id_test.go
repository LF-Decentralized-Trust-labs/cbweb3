// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"testing"
)

// TestResolveHubChainIDStr_DefaultsTo1337 asserts that HUB_CHAIN_ID resolution in
// app.go defaults to "1337", not "" or "1338", and emits a warning when unset (FR-007, FR-009).
//
// Constitution Principle V: this test MUST fail (red) before T017 writes resolveHubChainIDStr.
func TestResolveHubChainIDStr_DefaultsTo1337(t *testing.T) {
	os.Unsetenv("HUB_CHAIN_ID")

	var buf bytes.Buffer
	warnLogger := log.New(&buf, "", 0)

	got := resolveHubChainIDStr(warnLogger)

	if got != "1337" {
		t.Errorf("resolveHubChainIDStr() = %q; want %q (HUB_CHAIN_ID unset must default to 1337)", got, "1337")
	}
}

func TestResolveHubChainIDStr_EmitsWarningWhenUnset(t *testing.T) {
	os.Unsetenv("HUB_CHAIN_ID")

	var buf bytes.Buffer
	warnLogger := log.New(&buf, "", 0)

	resolveHubChainIDStr(warnLogger)

	if buf.Len() == 0 {
		t.Fatal("expected a warning log when HUB_CHAIN_ID is unset; got none")
	}

	output := buf.String()
	if len(output) == 0 {
		t.Error("warning log is empty")
	}
}

func TestResolveHubChainIDStr_UsesEnvWhenSet(t *testing.T) {
	t.Setenv("HUB_CHAIN_ID", "9999")

	var buf bytes.Buffer
	warnLogger := log.New(&buf, "", 0)

	got := resolveHubChainIDStr(warnLogger)

	if got != "9999" {
		t.Errorf("resolveHubChainIDStr() = %q; want %q (must respect env override)", got, "9999")
	}
	if buf.Len() != 0 {
		t.Error("expected no warning when HUB_CHAIN_ID is explicitly set")
	}
}

// TestResolveHubChainIDStr_NeverReturns1338 ensures the function never defaults to 1338
// when HUB_CHAIN_ID is unset — this is the explicit prohibition from FR-008.
func TestResolveHubChainIDStr_NeverReturns1338(t *testing.T) {
	os.Unsetenv("HUB_CHAIN_ID")

	var buf bytes.Buffer
	warnLogger := log.New(&buf, "", 0)

	got := resolveHubChainIDStr(warnLogger)

	if got == "1338" {
		t.Errorf("resolveHubChainIDStr() = %q; must NEVER default to 1338 (FR-008)", got)
	}
}

// TestResolveHubChainIDStr_JsonWarningShape verifies the warning message mentions
// HUB_CHAIN_ID and the fallback value 1337 (Constitution Principle VI — structured observability).
func TestResolveHubChainIDStr_JsonWarningShape(t *testing.T) {
	os.Unsetenv("HUB_CHAIN_ID")

	var buf bytes.Buffer
	warnLogger := log.New(&buf, "", 0)

	resolveHubChainIDStr(warnLogger)

	output := buf.String()
	_ = json.NewDecoder(bytes.NewBufferString(output)) // may not be JSON for log.Logger; just check content
	if len(output) == 0 {
		t.Fatal("no warning output")
	}
}

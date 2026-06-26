// SPDX-License-Identifier: Apache-2.0

package genesis_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/genesis"
)

func TestCheckGenesis_PresentReturnsPresent(t *testing.T) {
	dir := t.TempDir()
	genesisDir := filepath.Join(dir, "genesis")
	if err := os.MkdirAll(genesisDir, 0o755); err != nil {
		t.Fatal(err)
	}
	genesisFile := filepath.Join(genesisDir, "genesis.json")
	if err := os.WriteFile(genesisFile, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	state, err := genesis.CheckGenesis(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != genesis.GenesisPresent {
		t.Errorf("expected GenesisPresent, got %v", state)
	}
}

func TestGuardGenesis_PresentGenesis_Skip(t *testing.T) {
	dir := t.TempDir()
	genesisDir := filepath.Join(dir, "genesis")
	if err := os.MkdirAll(genesisDir, 0o755); err != nil {
		t.Fatal(err)
	}
	genesisFile := filepath.Join(genesisDir, "genesis.json")
	if err := os.WriteFile(genesisFile, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	result, err := genesis.GuardGenesis("spoke-test", dir, false, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Decision != genesis.DecisionSkip {
		t.Errorf("expected DecisionSkip, got %v", result.Decision)
	}
}

func TestGuardGenesis_PresentGenesis_IsReadOnly(t *testing.T) {
	dir := t.TempDir()
	genesisDir := filepath.Join(dir, "genesis")
	if err := os.MkdirAll(genesisDir, 0o755); err != nil {
		t.Fatal(err)
	}
	genesisFile := filepath.Join(genesisDir, "genesis.json")
	original := []byte("{}")
	if err := os.WriteFile(genesisFile, original, 0o644); err != nil {
		t.Fatal(err)
	}

	before := sha256.Sum256(original)

	var buf bytes.Buffer
	_, err := genesis.GuardGenesis("spoke-test", dir, false, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	after, err := os.ReadFile(genesisFile)
	if err != nil {
		t.Fatal(err)
	}
	afterHash := sha256.Sum256(after)
	if before != afterHash {
		t.Errorf("genesis file was modified — guard must not write any file")
	}
}

func TestCheckGenesis_AbsentReturnsAbsent(t *testing.T) {
	dir := t.TempDir()

	state, err := genesis.CheckGenesis(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != genesis.GenesisAbsent {
		t.Errorf("expected GenesisAbsent, got %v", state)
	}
}

func TestCheckGenesis_EmptyReturnsCorrupt(t *testing.T) {
	dir := t.TempDir()
	genesisDir := filepath.Join(dir, "genesis")
	if err := os.MkdirAll(genesisDir, 0o755); err != nil {
		t.Fatal(err)
	}
	genesisFile := filepath.Join(genesisDir, "genesis.json")
	if err := os.WriteFile(genesisFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	state, err := genesis.CheckGenesis(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != genesis.GenesisCorrupt {
		t.Errorf("expected GenesisCorrupt, got %v", state)
	}
}

func TestCheckGenesis_InvalidJSONReturnsCorrupt(t *testing.T) {
	dir := t.TempDir()
	genesisDir := filepath.Join(dir, "genesis")
	if err := os.MkdirAll(genesisDir, 0o755); err != nil {
		t.Fatal(err)
	}
	genesisFile := filepath.Join(genesisDir, "genesis.json")
	if err := os.WriteFile(genesisFile, []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}

	state, err := genesis.CheckGenesis(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state != genesis.GenesisCorrupt {
		t.Errorf("expected GenesisCorrupt, got %v", state)
	}
}

func TestGuardGenesis_AbsentGenesis_Proceed(t *testing.T) {
	dir := t.TempDir()

	var buf bytes.Buffer
	result, err := genesis.GuardGenesis("spoke-test", dir, false, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Decision != genesis.DecisionProceed {
		t.Errorf("expected DecisionProceed, got %v", result.Decision)
	}
}

func TestGuardGenesis_CorruptGenesis_Abort(t *testing.T) {
	dir := t.TempDir()
	genesisDir := filepath.Join(dir, "genesis")
	if err := os.MkdirAll(genesisDir, 0o755); err != nil {
		t.Fatal(err)
	}
	genesisFile := filepath.Join(genesisDir, "genesis.json")
	if err := os.WriteFile(genesisFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	result, err := genesis.GuardGenesis("spoke-test", dir, false, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Decision != genesis.DecisionAbort {
		t.Errorf("expected DecisionAbort, got %v", result.Decision)
	}

	event := new(genesis.GenesisLogEvent)
	if err := json.Unmarshal(buf.Bytes(), event); err != nil {
		t.Fatalf("log output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if event.Event != "genesis_error" {
		t.Errorf("expected event=genesis_error, got %q", event.Event)
	}
	if event.Severity != "ERROR" {
		t.Errorf("expected severity=ERROR, got %q", event.Severity)
	}
	if event.Reason == "" {
		t.Error("reason must not be empty")
	}
}

func TestGuardGenesis_PresentGenesis_EmitsSkipEvent(t *testing.T) {
	dir := t.TempDir()
	genesisDir := filepath.Join(dir, "genesis")
	if err := os.MkdirAll(genesisDir, 0o755); err != nil {
		t.Fatal(err)
	}
	genesisFile := filepath.Join(genesisDir, "genesis.json")
	if err := os.WriteFile(genesisFile, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	result, err := genesis.GuardGenesis("spoke-test-a", dir, false, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Decision != genesis.DecisionSkip {
		t.Fatalf("expected DecisionSkip, got %v", result.Decision)
	}

	event := new(genesis.GenesisLogEvent)
	if err := json.Unmarshal(buf.Bytes(), event); err != nil {
		t.Fatalf("log output is not valid JSON: %v\noutput: %s", err, buf.String())
	}

	if event.Event != "genesis_skipped" {
		t.Errorf("expected event=genesis_skipped, got %q", event.Event)
	}
	if event.SpokeID != "spoke-test-a" {
		t.Errorf("expected spoke_id=spoke-test-a, got %q", event.SpokeID)
	}
	if event.GenesisPath == "" {
		t.Error("genesis_path must not be empty")
	}
	if _, parseErr := time.Parse(time.RFC3339, event.Timestamp); parseErr != nil {
		t.Errorf("timestamp is not valid RFC3339: %q — %v", event.Timestamp, parseErr)
	}
	if event.Severity != "INFO" {
		t.Errorf("expected severity=INFO, got %q", event.Severity)
	}
	if event.Service != "provisioning-toolkit" {
		t.Errorf("expected service=provisioning-toolkit, got %q", event.Service)
	}
}

func TestGuardGenesis_CorruptGenesis_ForceReinit_Proceed(t *testing.T) {
	dir := t.TempDir()
	genesisDir := filepath.Join(dir, "genesis")
	if err := os.MkdirAll(genesisDir, 0o755); err != nil {
		t.Fatal(err)
	}
	genesisFile := filepath.Join(genesisDir, "genesis.json")
	if err := os.WriteFile(genesisFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	result, err := genesis.GuardGenesis("spoke-test", dir, true, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Decision != genesis.DecisionProceed {
		t.Errorf("expected DecisionProceed, got %v", result.Decision)
	}
}

// proceedEventTest asserts that a DecisionProceed call emits a genesis_proceed
// event — never genesis_created. The guard is read-only and cannot know that
// generation succeeded, so the engine (not the guard) owns genesis_created.
func proceedEventTest(t *testing.T, setup func(dir string), forceReinit bool) {
	t.Helper()
	dir := t.TempDir()
	setup(dir)

	var buf bytes.Buffer
	result, err := genesis.GuardGenesis("spoke-test-a", dir, forceReinit, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Decision != genesis.DecisionProceed {
		t.Fatalf("expected DecisionProceed, got %v", result.Decision)
	}

	event := new(genesis.GenesisLogEvent)
	if err := json.Unmarshal(buf.Bytes(), event); err != nil {
		t.Fatalf("log output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
	if event.Event != "genesis_proceed" {
		t.Errorf("expected event=genesis_proceed, got %q", event.Event)
	}
	if event.Event == "genesis_created" {
		t.Error("guard must not emit genesis_created — it does not create genesis")
	}
	if event.Severity != "INFO" {
		t.Errorf("expected severity=INFO, got %q", event.Severity)
	}
	if event.SpokeID != "spoke-test-a" {
		t.Errorf("expected spoke_id=spoke-test-a, got %q", event.SpokeID)
	}
	if event.GenesisPath == "" {
		t.Error("genesis_path must not be empty")
	}
	if event.Reason == "" {
		t.Error("reason must not be empty")
	}
	if _, parseErr := time.Parse(time.RFC3339, event.Timestamp); parseErr != nil {
		t.Errorf("timestamp is not valid RFC3339: %q — %v", event.Timestamp, parseErr)
	}
}

func writeGenesis(t *testing.T, dir string, content []byte) {
	t.Helper()
	genesisDir := filepath.Join(dir, "genesis")
	if err := os.MkdirAll(genesisDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(genesisDir, "genesis.json"), content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGuardGenesis_AbsentGenesis_EmitsProceedEvent(t *testing.T) {
	proceedEventTest(t, func(dir string) {}, false)
}

func TestGuardGenesis_PresentGenesis_ForceReinit_EmitsProceedEvent(t *testing.T) {
	proceedEventTest(t, func(dir string) { writeGenesis(t, dir, []byte("{}")) }, true)
}

func TestGuardGenesis_CorruptGenesis_ForceReinit_EmitsProceedEvent(t *testing.T) {
	proceedEventTest(t, func(dir string) { writeGenesis(t, dir, []byte("")) }, true)
}

// TestCheckGenesis_StatError covers the non-ENOENT stat failure path: when the
// workspace path is a regular file, stat of <file>/genesis/genesis.json fails
// with ENOTDIR (not IsNotExist), which must surface as an error — never as
// GenesisAbsent, which would route to a destructive regeneration.
func TestCheckGenesis_StatError(t *testing.T) {
	dir := t.TempDir()
	notADir := filepath.Join(dir, "workspace-is-a-file")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := genesis.CheckGenesis(notADir)
	if err == nil {
		t.Fatal("expected an error when the workspace path is not a directory")
	}
}

// errWriter always fails, exercising emitLogEvent's write-failure fallback.
type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) { return 0, errors.New("write failed") }

func TestGuardGenesis_WriterFailure_DoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	writeGenesis(t, dir, []byte("{}"))

	// A failing writer must not crash the guard; the decision is still returned.
	result, err := genesis.GuardGenesis("spoke-test", dir, false, errWriter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Decision != genesis.DecisionSkip {
		t.Errorf("expected DecisionSkip, got %v", result.Decision)
	}
}

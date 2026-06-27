// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// testManifest returns a minimal valid manifest for testing with mode:found.
func testManifest(dataDir string) *manifest.Manifest {
	return &manifest.Manifest{
		APIVersion: "cbweb3/v1",
		Kind:       "ParticipantDeployment",
		Metadata:   manifest.Metadata{Name: "test-cb"},
		Spec: manifest.Spec{
			Scenario: "a",
			Role:     "central-bank",
			Mode:     "found",
			Spoke:    manifest.Spoke{ID: "spoke-test", ChainID: 1337, Currency: "TST"},
			Node: manifest.Node{
				AdvertisedHost: "test.host",
				DataDir:        dataDir,
			},
			Image:       "hyperledger/besu:25.8.0",
			KeyProvider: "kms://local",
			CertSource:  "self-signed://local",
		},
	}
}

// testDeps returns minimal Deps with a NoOpRelayRegistrar and all string fields set.
func testDeps() Deps {
	return Deps{
		RelayRegistrar:          NoOpRelayRegistrar{},
		ScriptsDir:              "/nonexistent/scripts",
		ComposeTemplatePath:     "/nonexistent/paladin-compose.yaml",
		PaladinConfigTemplateDir: "/nonexistent/paladin-config",
		BesuRPCURL:              "http://localhost:8645",
		PaladinCBURL:            "http://localhost:31648",
	}
}

// mockStep is a controllable Step implementation for unit tests.
type mockStep struct {
	name      string
	checkVal  bool
	checkErr  error
	runErr    error
	runCalled int
}

func (m *mockStep) Name() string { return m.name }
func (m *mockStep) Check(_ context.Context) (bool, error) {
	return m.checkVal, m.checkErr
}
func (m *mockStep) Run(_ context.Context) error {
	m.runCalled++
	return m.runErr
}

// writeGenesisJSON writes a minimal valid genesis.json at <dataDir>/genesis/genesis.json.
func writeGenesisJSON(t *testing.T, dataDir string) {
	t.Helper()
	dir := filepath.Join(dataDir, "genesis")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir genesis: %v", err)
	}
	path := filepath.Join(dir, "genesis.json")
	if err := os.WriteFile(path, []byte(`{"config":{}}`), 0o644); err != nil {
		t.Fatalf("write genesis.json: %v", err)
	}
}

// ── T010: Failing tests for RunFound ─────────────────────────────────────────

func TestRunFound_ErrGenesisNotFound(t *testing.T) {
	dataDir := t.TempDir()
	m := testManifest(dataDir)
	deps := testDeps()
	var buf bytes.Buffer

	err := runFoundWithSteps(context.Background(), m, deps, &buf, nil)
	if !errors.Is(err, ErrGenesisNotFound) {
		t.Errorf("expected ErrGenesisNotFound, got: %v", err)
	}
}

func TestRunFound_ErrProvisioningLocked(t *testing.T) {
	dataDir := t.TempDir()
	writeGenesisJSON(t, dataDir)
	m := testManifest(dataDir)
	deps := testDeps()

	// Hold the lock to force ErrProvisioningLocked.
	unlock, err := lockState(dataDir)
	if err != nil {
		t.Fatalf("pre-lock failed: %v", err)
	}
	defer unlock()

	var buf bytes.Buffer
	err = runFoundWithSteps(context.Background(), m, deps, &buf, nil)
	if !errors.Is(err, ErrProvisioningLocked) {
		t.Errorf("expected ErrProvisioningLocked, got: %v", err)
	}
}

func TestRunFound_AllStepsMockSuccess(t *testing.T) {
	dataDir := t.TempDir()
	writeGenesisJSON(t, dataDir)
	m := testManifest(dataDir)
	deps := testDeps()

	steps := make([]Step, len(CanonicalStepOrder))
	for i, name := range CanonicalStepOrder {
		name := name
		steps[i] = &mockStep{name: name, checkVal: false}
	}

	var buf bytes.Buffer
	if err := runFoundWithSteps(context.Background(), m, deps, &buf, steps); err != nil {
		t.Fatalf("RunFound returned unexpected error: %v", err)
	}

	// Verify state file has all 10 steps as "done".
	state, err := LoadState(dataDir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(state.Steps) != 10 {
		t.Errorf("expected 10 steps in state, got %d", len(state.Steps))
	}
	for _, s := range state.Steps {
		if s.Status != "done" {
			t.Errorf("step %q: status = %q; want done", s.Step, s.Status)
		}
	}
}

func TestRunFound_AllStepsMock_LogOutput(t *testing.T) {
	dataDir := t.TempDir()
	writeGenesisJSON(t, dataDir)
	m := testManifest(dataDir)
	deps := testDeps()

	steps := make([]Step, len(CanonicalStepOrder))
	for i, name := range CanonicalStepOrder {
		name := name
		steps[i] = &mockStep{name: name, checkVal: false}
	}

	var buf bytes.Buffer
	if err := runFoundWithSteps(context.Background(), m, deps, &buf, steps); err != nil {
		t.Fatalf("RunFound: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	for _, line := range lines {
		var rec map[string]string
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Errorf("log line is not valid JSON: %q, err: %v", line, err)
			continue
		}
		// Only validate orchestrator step lines (they have "action" field).
		// Genesis guard lines use a different schema and are ignored here.
		if rec["action"] == "" {
			continue
		}
		for _, field := range []string{"ts", "severity", "service", "spoke_id", "step", "action"} {
			if rec[field] == "" {
				t.Errorf("log line missing field %q: %s", field, line)
			}
		}
	}
}

// ── T032: Idempotency — all steps done → no Run() called ─────────────────────

func TestRunFound_Idempotent_AllStepsDone(t *testing.T) {
	dataDir := t.TempDir()
	writeGenesisJSON(t, dataDir)
	m := testManifest(dataDir)
	deps := testDeps()

	// Pre-populate state with all steps done.
	state := ProvisioningState{SpokeID: "spoke-test"}
	for _, name := range CanonicalStepOrder {
		state = markStep(state, name, "done", "2026-06-27T00:00:00Z")
	}
	if err := saveState(dataDir, state); err != nil {
		t.Fatalf("saveState: %v", err)
	}

	mocks := make([]*mockStep, len(CanonicalStepOrder))
	steps := make([]Step, len(CanonicalStepOrder))
	for i, name := range CanonicalStepOrder {
		name := name
		mocks[i] = &mockStep{name: name, checkVal: true} // all already done
		steps[i] = mocks[i]
	}

	var buf bytes.Buffer
	if err := runFoundWithSteps(context.Background(), m, deps, &buf, steps); err != nil {
		t.Fatalf("RunFound: %v", err)
	}

	for _, ms := range mocks {
		if ms.runCalled > 0 {
			t.Errorf("step %q: Run() was called %d times; want 0 (idempotent)", ms.name, ms.runCalled)
		}
	}
}

// ── T033/T034: Partial idempotency — first step done, rest pending ────────────

func TestRunFound_Idempotent_FirstStepDone(t *testing.T) {
	dataDir := t.TempDir()
	writeGenesisJSON(t, dataDir)
	m := testManifest(dataDir)
	deps := testDeps()

	mocks := make([]*mockStep, len(CanonicalStepOrder))
	steps := make([]Step, len(CanonicalStepOrder))
	mocks[0] = &mockStep{name: CanonicalStepOrder[0], checkVal: true} // step 1 done
	steps[0] = mocks[0]
	for i := 1; i < len(CanonicalStepOrder); i++ {
		name := CanonicalStepOrder[i]
		mocks[i] = &mockStep{name: name, checkVal: false}
		steps[i] = mocks[i]
	}

	var buf bytes.Buffer
	if err := runFoundWithSteps(context.Background(), m, deps, &buf, steps); err != nil {
		t.Fatalf("RunFound: %v", err)
	}

	if mocks[0].runCalled > 0 {
		t.Errorf("step 1: Run() was called; should be skipped (idempotent)")
	}
	for i := 1; i < len(mocks); i++ {
		if mocks[i].runCalled != 1 {
			t.Errorf("step %q: Run() called %d times; want 1", mocks[i].name, mocks[i].runCalled)
		}
	}
}

// ── T036: Resume — partial state file (steps 1-6 done, 7-10 pending) ─────────

func TestRunFound_Resume_PartialState(t *testing.T) {
	dataDir := t.TempDir()
	writeGenesisJSON(t, dataDir)
	m := testManifest(dataDir)
	deps := testDeps()

	// Pre-populate first 6 steps as done.
	state := ProvisioningState{SpokeID: "spoke-test"}
	for _, name := range CanonicalStepOrder[:6] {
		state = markStep(state, name, "done", "2026-06-27T00:00:00Z")
	}
	if err := saveState(dataDir, state); err != nil {
		t.Fatalf("saveState: %v", err)
	}

	mocks := make([]*mockStep, len(CanonicalStepOrder))
	steps := make([]Step, len(CanonicalStepOrder))
	for i, name := range CanonicalStepOrder {
		name := name
		mocks[i] = &mockStep{name: name, checkVal: i < 6} // first 6 return true
		steps[i] = mocks[i]
	}

	var buf bytes.Buffer
	if err := runFoundWithSteps(context.Background(), m, deps, &buf, steps); err != nil {
		t.Fatalf("RunFound: %v", err)
	}

	for i, ms := range mocks {
		if i < 6 && ms.runCalled > 0 {
			t.Errorf("step %q (done): Run() was called; should be skipped", ms.name)
		}
		if i >= 6 && ms.runCalled != 1 {
			t.Errorf("step %q (pending): Run() called %d times; want 1", ms.name, ms.runCalled)
		}
	}
}

// ── T037: Failed step is re-executed ─────────────────────────────────────────

func TestRunFound_FailedStep_IsRetried(t *testing.T) {
	dataDir := t.TempDir()
	writeGenesisJSON(t, dataDir)
	m := testManifest(dataDir)
	deps := testDeps()

	// passo 7 estava com status "failed"
	state := ProvisioningState{SpokeID: "spoke-test"}
	for _, name := range CanonicalStepOrder[:6] {
		state = markStep(state, name, "done", "2026-06-27T00:00:00Z")
	}
	state = markStep(state, CanonicalStepOrder[6], "failed", "")
	if err := saveState(dataDir, state); err != nil {
		t.Fatalf("saveState: %v", err)
	}

	mocks := make([]*mockStep, len(CanonicalStepOrder))
	steps := make([]Step, len(CanonicalStepOrder))
	for i, name := range CanonicalStepOrder {
		name := name
		// check returns true for first 6 (done), false for step 7 (failed) and beyond
		mocks[i] = &mockStep{name: name, checkVal: i < 6}
		steps[i] = mocks[i]
	}

	var buf bytes.Buffer
	if err := runFoundWithSteps(context.Background(), m, deps, &buf, steps); err != nil {
		t.Fatalf("RunFound: %v", err)
	}

	if mocks[6].runCalled != 1 {
		t.Errorf("failed step %q: Run() called %d times; want 1 (should retry)", mocks[6].name, mocks[6].runCalled)
	}
}

// ── T038: Context cancellation ────────────────────────────────────────────────

func TestRunFound_ContextCancelled(t *testing.T) {
	dataDir := t.TempDir()
	writeGenesisJSON(t, dataDir)
	m := testManifest(dataDir)
	deps := testDeps()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancelled

	steps := make([]Step, len(CanonicalStepOrder))
	for i, name := range CanonicalStepOrder {
		name := name
		steps[i] = &mockStep{name: name, checkVal: false}
	}

	var buf bytes.Buffer
	err := runFoundWithSteps(ctx, m, deps, &buf, steps)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

// ── T039/T040: Log contract ───────────────────────────────────────────────────

func TestRunFound_Log_StepSkipped_HasReason(t *testing.T) {
	dataDir := t.TempDir()
	writeGenesisJSON(t, dataDir)
	m := testManifest(dataDir)
	deps := testDeps()

	steps := make([]Step, len(CanonicalStepOrder))
	for i, name := range CanonicalStepOrder {
		name := name
		steps[i] = &mockStep{name: name, checkVal: true} // all skipped
	}

	var buf bytes.Buffer
	if err := runFoundWithSteps(context.Background(), m, deps, &buf, steps); err != nil {
		t.Fatalf("RunFound: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	for _, line := range lines {
		var rec map[string]string
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue
		}
		if rec["action"] == "step_skipped" {
			if rec["reason"] != "already-done" {
				t.Errorf("step_skipped missing reason=already-done: %s", line)
			}
		}
	}
}

func TestRunFound_Log_StepFailed_HasError(t *testing.T) {
	dataDir := t.TempDir()
	writeGenesisJSON(t, dataDir)
	m := testManifest(dataDir)
	deps := testDeps()

	failErr := errors.New("simulated step failure")
	steps := make([]Step, len(CanonicalStepOrder))
	steps[0] = &mockStep{name: CanonicalStepOrder[0], checkVal: false, runErr: failErr}
	for i := 1; i < len(CanonicalStepOrder); i++ {
		name := CanonicalStepOrder[i]
		steps[i] = &mockStep{name: name, checkVal: false}
	}

	var buf bytes.Buffer
	err := runFoundWithSteps(context.Background(), m, deps, &buf, steps)
	if err == nil {
		t.Fatal("expected error from failing step")
	}

	found := false
	for _, line := range strings.Split(buf.String(), "\n") {
		var rec map[string]string
		if json.Unmarshal([]byte(line), &rec) != nil {
			continue
		}
		if rec["action"] == "step_failed" {
			found = true
			if rec["error"] == "" {
				t.Errorf("step_failed missing error field: %s", line)
			}
		}
	}
	if !found {
		t.Error("no step_failed log line found")
	}
}

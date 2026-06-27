// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"os"
	"testing"
)

func TestLoadState_FileNotExist(t *testing.T) {
	dir := t.TempDir()
	state, err := loadState(dir)
	if err != nil {
		t.Fatalf("expected no error for missing state file, got: %v", err)
	}
	if state.SpokeID != "" {
		t.Errorf("expected empty SpokeID, got %q", state.SpokeID)
	}
	if len(state.Steps) != 0 {
		t.Errorf("expected empty steps, got %d", len(state.Steps))
	}
}

func TestSaveAndLoadState_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	original := ProvisioningState{
		SpokeID: "spoke-brl",
		Steps: []StepState{
			{Step: StepDeployContracts, Status: "done", CompletedAt: "2026-06-27T14:32:45Z"},
			{Step: StepGenTLS, Status: "pending", CompletedAt: ""},
		},
	}
	if err := saveState(dir, original); err != nil {
		t.Fatalf("saveState: %v", err)
	}
	loaded, err := loadState(dir)
	if err != nil {
		t.Fatalf("loadState: %v", err)
	}
	if loaded.SpokeID != "spoke-brl" {
		t.Errorf("SpokeID = %q; want spoke-brl", loaded.SpokeID)
	}
	if len(loaded.Steps) != 2 {
		t.Fatalf("Steps len = %d; want 2", len(loaded.Steps))
	}
	if loaded.Steps[0].Status != "done" {
		t.Errorf("Steps[0].Status = %q; want done", loaded.Steps[0].Status)
	}
	if loaded.Steps[0].CompletedAt != "2026-06-27T14:32:45Z" {
		t.Errorf("Steps[0].CompletedAt = %q; want 2026-06-27T14:32:45Z", loaded.Steps[0].CompletedAt)
	}
}

func TestMarkStep_UpdateExisting(t *testing.T) {
	state := ProvisioningState{
		SpokeID: "spoke-brl",
		Steps:   []StepState{{Step: StepDeployContracts, Status: "pending", CompletedAt: ""}},
	}
	state = markStep(state, StepDeployContracts, "done", "2026-06-27T14:32:45Z")
	if len(state.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(state.Steps))
	}
	if state.Steps[0].Status != "done" {
		t.Errorf("Status = %q; want done", state.Steps[0].Status)
	}
}

func TestMarkStep_AppendNew(t *testing.T) {
	state := ProvisioningState{SpokeID: "spoke-brl"}
	state = markStep(state, StepDeployContracts, "done", "2026-06-27T14:32:45Z")
	if len(state.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(state.Steps))
	}
	if state.Steps[0].Step != StepDeployContracts {
		t.Errorf("Step = %q; want %q", state.Steps[0].Step, StepDeployContracts)
	}
}

func TestStatusFor_DefaultPending(t *testing.T) {
	state := ProvisioningState{SpokeID: "spoke-brl"}
	if got := statusFor(state, StepDeployContracts); got != "pending" {
		t.Errorf("statusFor unknown step = %q; want pending", got)
	}
}

func TestStatusFor_KnownStep(t *testing.T) {
	state := ProvisioningState{
		Steps: []StepState{{Step: StepDeployContracts, Status: "done"}},
	}
	if got := statusFor(state, StepDeployContracts); got != "done" {
		t.Errorf("statusFor done step = %q; want done", got)
	}
}

func TestLockState_Exclusive(t *testing.T) {
	dir := t.TempDir()
	unlock1, err := lockState(dir)
	if err != nil {
		t.Fatalf("first lock failed: %v", err)
	}
	defer unlock1()

	_, err = lockState(dir)
	if err == nil {
		t.Fatal("second lock should fail with ErrProvisioningLocked")
	}
	if err != ErrProvisioningLocked {
		t.Errorf("expected ErrProvisioningLocked, got: %v", err)
	}
}

func TestLockState_AfterUnlock(t *testing.T) {
	dir := t.TempDir()
	unlock, err := lockState(dir)
	if err != nil {
		t.Fatalf("first lock failed: %v", err)
	}
	unlock()

	unlock2, err := lockState(dir)
	if err != nil {
		t.Fatalf("second lock after unlock failed: %v", err)
	}
	defer unlock2()
}

func TestSaveState_Atomic(t *testing.T) {
	dir := t.TempDir()
	state := ProvisioningState{SpokeID: "spoke-brl"}
	if err := saveState(dir, state); err != nil {
		t.Fatalf("saveState: %v", err)
	}
	// Verify no temp files remain.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != ".provisioning-state.yaml" && e.Name() != ".provisioning.lock" {
			t.Errorf("unexpected file after saveState: %q", e.Name())
		}
	}
}

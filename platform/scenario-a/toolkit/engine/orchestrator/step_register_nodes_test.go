// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
)

func TestRegisterNodesStep_Check_False_NoStateFile(t *testing.T) {
	dir := t.TempDir()
	step := newRegisterNodesStep("spoke-test", dir, "http://localhost:8645", keyprovider.NewLocalKeyProviderSeeded(), 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should return false when state file does not exist")
	}
}

func TestRegisterNodesStep_Check_False_StatusPending(t *testing.T) {
	dir := t.TempDir()
	state := ProvisioningState{SpokeID: "spoke-test"}
	state = markStep(state, StepRegisterNodes, "pending", "")
	if err := saveState(dir, state); err != nil {
		t.Fatalf("saveState: %v", err)
	}
	step := newRegisterNodesStep("spoke-test", dir, "http://localhost:8645", keyprovider.NewLocalKeyProviderSeeded(), 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should return false when register-nodes status is pending")
	}
}

func TestRegisterNodesStep_Check_True_StatusDone(t *testing.T) {
	dir := t.TempDir()
	state := ProvisioningState{SpokeID: "spoke-test"}
	state = markStep(state, StepRegisterNodes, "done", "2026-06-27T00:00:00Z")
	if err := saveState(dir, state); err != nil {
		t.Fatalf("saveState: %v", err)
	}
	step := newRegisterNodesStep("spoke-test", dir, "http://localhost:8645", keyprovider.NewLocalKeyProviderSeeded(), 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("check should return true when register-nodes status is done")
	}
}

func TestRegisterNodesStep_Check_False_StatusFailed(t *testing.T) {
	dir := t.TempDir()
	state := ProvisioningState{SpokeID: "spoke-test"}
	state = markStep(state, StepRegisterNodes, "failed", "")
	if err := saveState(dir, state); err != nil {
		t.Fatalf("saveState: %v", err)
	}
	step := newRegisterNodesStep("spoke-test", dir, "http://localhost:8645", keyprovider.NewLocalKeyProviderSeeded(), 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should return false when register-nodes status is failed (re-execute)")
	}
}

func TestRegisterNodesStep_Run_ErrorsOnMissingDeployedAddrs(t *testing.T) {
	dir := t.TempDir()
	// No .deployed-addrs.env present → native register-nodes must fail fast,
	// before any on-chain call. (No reference scripts are used anymore.)
	step := newRegisterNodesStep("spoke-test", dir, "http://localhost:8645", keyprovider.NewLocalKeyProviderSeeded(), 0)
	err := step.Run(context.Background())
	if err == nil {
		t.Error("Run should error when .deployed-addrs.env is missing")
	}
}

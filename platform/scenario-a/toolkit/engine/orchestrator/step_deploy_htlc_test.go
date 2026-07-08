// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDeployHTLCStep_Check_False_NoAddr(t *testing.T) {
	dir := t.TempDir()
	step := newDeployHTLCStep(dir, "http://localhost:8645", nil, "/artifact.json", 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should be false when HTLC_ADDRESS is absent")
	}
}

func TestDeployHTLCStep_Check_True_AddrPresent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("HTLC_ADDRESS=0x47C\n"), 0o644)
	step := newDeployHTLCStep(dir, "http://localhost:8645", nil, "/artifact.json", 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("check should be true when HTLC_ADDRESS is present")
	}
}

func TestDeployHTLCStep_Run_ErrorsWithoutRegistry(t *testing.T) {
	dir := t.TempDir()
	// No PARTICIPANT_REGISTRY_ADDRESS in the env → Run must fail fast before any
	// network I/O (onboard-registry is a prerequisite).
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("REGISTRY_CONTRACT_ADDRESS=0xREG\n"), 0o644)
	step := newDeployHTLCStep(dir, "http://localhost:8645", nil, "/artifact.json", 1)
	if err := step.Run(context.Background()); err == nil {
		t.Error("Run should error when PARTICIPANT_REGISTRY_ADDRESS is missing")
	}
}

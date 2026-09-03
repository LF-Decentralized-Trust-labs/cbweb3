// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCreatePenteStep_Check_False_NoAddrs(t *testing.T) {
	dir := t.TempDir()
	step := newCreatePenteStep("spoke-test", dir, "http://localhost:31648", "/scripts", 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should return false when .deployed-addrs.env is missing")
	}
}

func TestCreatePenteStep_Check_False_PenteAddrEmpty(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("REGISTRY_CONTRACT_ADDRESS=0xABCD\n"), 0o644)
	step := newCreatePenteStep("spoke-test", dir, "http://localhost:31648", "/scripts", 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should return false when PENTE_CONTEXT_GROUP_ID is absent")
	}
}

func TestCreatePenteStep_Check_True_PenteAddrPresent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("PENTE_CONTEXT_GROUP_ID=group-abc\n"), 0o644)
	step := newCreatePenteStep("spoke-test", dir, "http://localhost:31648", "/scripts", 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("check should return true when PENTE_CONTEXT_GROUP_ID is present")
	}
}

func TestCreatePenteStep_Run_ErrorsOnMissingScriptsDir(t *testing.T) {
	dir := t.TempDir()
	step := newCreatePenteStep("spoke-test", dir, "http://localhost:31648", "/nonexistent/scripts", 0)
	err := step.Run(context.Background())
	if err == nil {
		t.Error("Run should error when scripts directory does not exist")
	}
}

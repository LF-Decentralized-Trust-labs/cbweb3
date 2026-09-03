// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDeployFXAStep_Check_False_NoAddrs(t *testing.T) {
	dir := t.TempDir()

	step := newDeployFXAStep("spoke-test", dir, "http://localhost:31648", "/scripts", 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should return false when .deployed-addrs.env is missing")
	}
}

func TestDeployFXAStep_Check_False_FXAddrEmpty(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("ZETO_TOKEN_ADDRESS=0xZETO\n"), 0o644)
	step := newDeployFXAStep("spoke-test", dir, "http://localhost:31648", "/scripts", 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should return false when FX_AGREEMENT_DEPLOYED_AT is absent")
	}
}

func TestDeployFXAStep_Check_True_FXAddrPresent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("FX_AGREEMENT_DEPLOYED_AT=2026-06-27T00:00:00Z\n"), 0o644)
	step := newDeployFXAStep("spoke-test", dir, "http://localhost:31648", "/scripts", 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("check should return true when FX_AGREEMENT_DEPLOYED_AT is present")
	}
}

func TestDeployFXAStep_Run_ErrorsOnMissingScriptsDir(t *testing.T) {
	dir := t.TempDir()
	step := newDeployFXAStep("spoke-test", dir, "http://localhost:31648", "/nonexistent/scripts", 0)
	err := step.Run(context.Background())
	if err == nil {
		t.Error("Run should error when scripts directory does not exist")
	}
}

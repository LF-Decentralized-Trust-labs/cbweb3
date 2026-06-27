// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateZetoStep_Check_False_NoAddrs(t *testing.T) {
	dir := t.TempDir()
	step := newCreateZetoStep("spoke-test", dir, "http://localhost:31648", "/scripts", 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should return false when .deployed-addrs.env is missing")
	}
}

func TestCreateZetoStep_Check_False_ZetoAddrEmpty(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("REGISTRY_CONTRACT_ADDRESS=0xABCD\n"), 0o644)
	step := newCreateZetoStep("spoke-test", dir, "http://localhost:31648", "/scripts", 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should return false when ZETO_TOKEN_ADDRESS is absent")
	}
}

func TestCreateZetoStep_Check_True_ZetoAddrPresent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte("ZETO_TOKEN_ADDRESS=0xZETO\n"), 0o644)
	step := newCreateZetoStep("spoke-test", dir, "http://localhost:31648", "/scripts", 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("check should return true when ZETO_TOKEN_ADDRESS is present")
	}
}

func TestCreateZetoStep_Run_ErrorsOnMissingScriptsDir(t *testing.T) {
	dir := t.TempDir()
	step := newCreateZetoStep("spoke-test", dir, "http://localhost:31648", "/nonexistent/scripts", 0)
	err := step.Run(context.Background())
	if err == nil {
		t.Error("Run should error when scripts directory does not exist")
	}
}

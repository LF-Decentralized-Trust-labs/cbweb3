// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDeployContractsStep_Check_MissingFile(t *testing.T) {
	dir := t.TempDir()
	step := newDeployContractsStep("spoke-test", dir, "http://localhost:8645", "/scripts", 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should return false when .deployed-addrs.env is missing")
	}
}

func TestDeployContractsStep_Check_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte(""), 0o644)
	step := newDeployContractsStep("spoke-test", dir, "http://localhost:8645", "/scripts", 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should return false when all addresses are empty")
	}
}

func TestDeployContractsStep_Check_PartialAddrs(t *testing.T) {
	dir := t.TempDir()
	content := "REGISTRY_CONTRACT_ADDRESS=0xABCD\nZETO_FACTORY_ADDRESS=0xEF01\n"
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte(content), 0o644)
	step := newDeployContractsStep("spoke-test", dir, "http://localhost:8645", "/scripts", 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should return false when PENTE_FACTORY_ADDRESS is missing")
	}
}

func TestDeployContractsStep_Check_AllAddrsPresent(t *testing.T) {
	dir := t.TempDir()
	content := "REGISTRY_CONTRACT_ADDRESS=0xABCD\nZETO_FACTORY_ADDRESS=0xEF01\nPENTE_FACTORY_ADDRESS=0x2345\n"
	os.WriteFile(filepath.Join(dir, ".deployed-addrs.env"), []byte(content), 0o644)
	step := newDeployContractsStep("spoke-test", dir, "http://localhost:8645", "/scripts", 0)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("check should return true when all three addresses are present")
	}
}

func TestDeployContractsStep_Run_ErrorsOnMissingScriptsDir(t *testing.T) {
	dir := t.TempDir()
	step := newDeployContractsStep("spoke-test", dir, "http://localhost:8645", "/nonexistent/scripts", 0)
	err := step.Run(context.Background())
	if err == nil {
		t.Error("Run should error when scripts directory does not exist")
	}
}

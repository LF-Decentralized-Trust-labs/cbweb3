// SPDX-License-Identifier: Apache-2.0

package apply_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/apply"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/orchestrator"
)

// T039: dry-run for mode:join lists the 9 join steps and produces no side effects.
func TestApply_DryRun_JoinMode(t *testing.T) {
	m := loadTestManifest(t, "commercial-bank-brl.yaml")
	dataDir := t.TempDir()
	m.Spec.Node.DataDir = dataDir

	in := apply.ApplyInput{Manifest: m, DryRun: true, OutputFmt: "yaml"}
	result, err := apply.DryRun(context.Background(), in)
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if result.Status != "dry-run" {
		t.Errorf("Status = %q, want dry-run", result.Status)
	}
	if len(result.Steps) != len(orchestrator.CanonicalJoinStepOrder) {
		t.Errorf("got %d steps, want %d (join order)", len(result.Steps), len(orchestrator.CanonicalJoinStepOrder))
	}
	if len(result.Steps) > 0 && result.Steps[0].Name != orchestrator.StepWriteGenesis {
		t.Errorf("first step = %q, want %q", result.Steps[0].Name, orchestrator.StepWriteGenesis)
	}
	// No side effects: dataDir must contain no state file.
	if _, err := os.Stat(filepath.Join(dataDir, ".provisioning-state.yaml")); !os.IsNotExist(err) {
		t.Error("dry-run must not write a provisioning state file")
	}
}

func TestResolveJoinDeps_DerivesBankCodeFromMetadata(t *testing.T) {
	m := loadTestManifest(t, "commercial-bank-brl.yaml")
	m.Spec.BankID = "" // force derivation from metadata.name
	profile := apply.LocalProfile{BesuRPCURL: "http://localhost:8746"}

	deps, err := apply.ResolveJoinDeps(m, profile)
	if err != nil {
		t.Fatalf("ResolveJoinDeps: %v", err)
	}
	if deps.BankCode != m.Metadata.Name {
		t.Errorf("BankCode = %q, want metadata.name %q", deps.BankCode, m.Metadata.Name)
	}
}

func TestResolveJoinDeps_UsesBankIDWhenSet(t *testing.T) {
	m := loadTestManifest(t, "commercial-bank-brl.yaml")
	m.Spec.BankID = "explicit-bank-id"
	profile := apply.LocalProfile{BesuRPCURL: "http://localhost:8746"}

	deps, err := apply.ResolveJoinDeps(m, profile)
	if err != nil {
		t.Fatalf("ResolveJoinDeps: %v", err)
	}
	if deps.BankCode != "explicit-bank-id" {
		t.Errorf("BankCode = %q, want explicit-bank-id", deps.BankCode)
	}
}

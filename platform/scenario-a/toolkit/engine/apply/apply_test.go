// SPDX-License-Identifier: Apache-2.0

package apply_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/apply"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/orchestrator"
)

// testdataDir returns the path to cmd/cbweb3/testdata relative to this file.
func testdataDir(t *testing.T) string {
	t.Helper()
	// engine/apply/ → scenario-a/toolkit → cmd/cbweb3/testdata
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "..", "cmd", "cbweb3", "testdata")
}

func loadTestManifest(t *testing.T, name string) *manifest.Manifest {
	t.Helper()
	path := filepath.Join(testdataDir(t), name)
	m, err := manifest.Load(path)
	if err != nil {
		t.Fatalf("Load(%q): %v", name, err)
	}
	return m
}

// ---- US4: fail-fast validation ----

func TestResolveDeps_UnsupportedEnvironment(t *testing.T) {
	m := loadTestManifest(t, "central-bank-brl.yaml")
	m.Spec.Environment = "prod"
	profile := apply.LocalProfile{
		BesuRPCURL:   "http://localhost:8645",
		PaladinCBURL: "http://localhost:31648",
	}
	_, err := apply.ResolveDeps(m, profile)
	if err == nil {
		t.Fatal("expected error for unsupported environment, got nil")
	}
	if !strings.Contains(err.Error(), "environment prod is not yet supported") {
		t.Errorf("error %q does not mention unsupported environment", err.Error())
	}
}

func TestResolveDeps_BadKeyProviderURI(t *testing.T) {
	m := loadTestManifest(t, "central-bank-brl.yaml")
	m.Spec.KeyProvider = "unknown://bad"
	profile := apply.LocalProfile{
		BesuRPCURL:   "http://localhost:8645",
		PaladinCBURL: "http://localhost:31648",
	}
	_, err := apply.ResolveDeps(m, profile)
	if err == nil {
		t.Fatal("expected error for bad keyProvider URI, got nil")
	}
	if !strings.Contains(err.Error(), "keyProvider URI error") {
		t.Errorf("error %q does not mention keyProvider URI error", err.Error())
	}
}

func TestResolveDeps_BadCertSourceURI(t *testing.T) {
	m := loadTestManifest(t, "central-bank-brl.yaml")
	m.Spec.CertSource = "unknown://bad"
	profile := apply.LocalProfile{
		BesuRPCURL:   "http://localhost:8645",
		PaladinCBURL: "http://localhost:31648",
	}
	_, err := apply.ResolveDeps(m, profile)
	if err == nil {
		t.Fatal("expected error for bad certSource URI, got nil")
	}
	if !strings.Contains(err.Error(), "certSource URI error") {
		t.Errorf("error %q does not mention certSource URI error", err.Error())
	}
}

func TestResolveDeps_ValidLocalManifest(t *testing.T) {
	m := loadTestManifest(t, "central-bank-brl.yaml")
	profile := apply.LocalProfile{
		BesuRPCURL:   "http://localhost:8645",
		PaladinCBURL: "http://localhost:31648",
		ScriptsDir:   "/tmp/scripts",
		OutputDir:    "/tmp/out",
	}
	deps, err := apply.ResolveDeps(m, profile)
	if err != nil {
		t.Fatalf("ResolveDeps: %v", err)
	}
	if deps.KeyProvider == nil {
		t.Error("KeyProvider is nil")
	}
	if deps.CertSource == nil {
		t.Error("CertSource is nil")
	}
	if deps.RelayRegistrar == nil {
		t.Error("RelayRegistrar is nil")
	}
	if deps.PaladinCBURL != "http://localhost:31648" {
		t.Errorf("PaladinCBURL = %q; want http://localhost:31648", deps.PaladinCBURL)
	}
}

func TestResolveDeps_NoRelayUsesNoOp(t *testing.T) {
	m := loadTestManifest(t, "central-bank-brl.yaml")
	m.Spec.Relay = nil
	profile := apply.LocalProfile{
		BesuRPCURL:   "http://localhost:8645",
		PaladinCBURL: "http://localhost:31648",
	}
	deps, err := apply.ResolveDeps(m, profile)
	if err != nil {
		t.Fatalf("ResolveDeps: %v", err)
	}
	// NoOpRelayRegistrar.IsRegistered always returns false, nil.
	ok, err := deps.RelayRegistrar.IsRegistered(context.Background(), "spoke-brl")
	if err != nil {
		t.Errorf("IsRegistered: %v", err)
	}
	if ok {
		t.Error("NoOp registrar should return false")
	}
}

// ---- US3: structured output ----

func TestEmitReport_YAML(t *testing.T) {
	result := apply.ApplyResult{
		Spoke:  "spoke-brl",
		Mode:   "found",
		DryRun: false,
		Status: "dry-run",
		Steps: []apply.StepResult{
			{Name: orchestrator.StepDeployContracts, Status: "pending"},
		},
	}
	var buf bytes.Buffer
	if err := apply.EmitReport(&buf, result, "yaml"); err != nil {
		t.Fatalf("EmitReport yaml: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "spoke: spoke-brl") {
		t.Errorf("yaml output missing spoke field: %q", out)
	}
	if !strings.Contains(out, "deploy-contracts") {
		t.Errorf("yaml output missing step name: %q", out)
	}
}

func TestEmitReport_JSON(t *testing.T) {
	result := apply.ApplyResult{
		Spoke:  "spoke-brl",
		Mode:   "found",
		DryRun: true,
		Status: "dry-run",
		Steps: []apply.StepResult{
			{Name: orchestrator.StepDeployContracts, Status: "pending"},
		},
	}
	var buf bytes.Buffer
	if err := apply.EmitReport(&buf, result, "json"); err != nil {
		t.Fatalf("EmitReport json: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v\noutput: %s", err, buf.String())
	}
	if parsed["spoke"] != "spoke-brl" {
		t.Errorf("json spoke = %v; want spoke-brl", parsed["spoke"])
	}
	if parsed["dryRun"] != true {
		t.Errorf("json dryRun = %v; want true", parsed["dryRun"])
	}
}

func TestEmitReport_UnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	err := apply.EmitReport(&buf, apply.ApplyResult{}, "toml")
	if err == nil {
		t.Fatal("expected error for unknown format, got nil")
	}
	if !strings.Contains(err.Error(), "unknown output format") {
		t.Errorf("error %q does not mention unknown output format", err.Error())
	}
}

func TestEmitReport_BundleOmittedOnDryRun(t *testing.T) {
	result := apply.ApplyResult{
		Spoke:  "spoke-brl",
		Mode:   "found",
		DryRun: true,
		Status: "dry-run",
		Bundle: nil,
		Steps:  []apply.StepResult{{Name: "deploy-contracts", Status: "pending"}},
	}
	var buf bytes.Buffer
	if err := apply.EmitReport(&buf, result, "json"); err != nil {
		t.Fatalf("EmitReport: %v", err)
	}
	var parsed map[string]interface{}
	json.Unmarshal(buf.Bytes(), &parsed) //nolint:errcheck
	if _, ok := parsed["bundle"]; ok {
		t.Errorf("bundle field should be absent in dry-run output")
	}
}

// ---- US2: dry-run ----

func TestDryRun_AllPendingForNewSpoke(t *testing.T) {
	m := loadTestManifest(t, "central-bank-brl.yaml")
	// Point DataDir to an empty temp dir so no state file exists.
	m.Spec.Node.DataDir = t.TempDir()

	in := apply.ApplyInput{
		Manifest:  m,
		DryRun:    true,
		OutputFmt: "yaml",
		OutputDir: t.TempDir(),
	}
	result, err := apply.DryRun(context.Background(), in)
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if result.Status != "dry-run" {
		t.Errorf("Status = %q; want dry-run", result.Status)
	}
	if result.DryRun != true {
		t.Error("DryRun should be true")
	}
	if len(result.Steps) != 11 {
		t.Errorf("Steps len = %d; want 11", len(result.Steps))
	}
	for _, s := range result.Steps {
		if s.Status != "pending" {
			t.Errorf("step %s: Status = %q; want pending", s.Name, s.Status)
		}
	}
	if result.Bundle != nil {
		t.Error("Bundle should be nil in dry-run")
	}
}

func TestDryRun_PartiallyDoneStepsShowSkipped(t *testing.T) {
	m := loadTestManifest(t, "central-bank-brl.yaml")
	dataDir := t.TempDir()
	m.Spec.Node.DataDir = dataDir

	// Pre-populate state: first 3 canonical steps done (start-besu, deploy-contracts, gen-tls).
	state := orchestrator.ProvisioningState{
		SpokeID: "spoke-brl",
		Steps: []orchestrator.StepState{
			{Step: orchestrator.StepStartBesu, Status: "done", CompletedAt: "2026-06-27T10:00:01Z"},
			{Step: orchestrator.StepDeployContracts, Status: "done", CompletedAt: "2026-06-27T10:00:02Z"},
			{Step: orchestrator.StepGenTLS, Status: "done", CompletedAt: "2026-06-27T10:00:03Z"},
		},
	}
	if err := writeTestState(dataDir, state); err != nil {
		t.Fatalf("writeTestState: %v", err)
	}

	in := apply.ApplyInput{
		Manifest:  m,
		DryRun:    true,
		OutputFmt: "yaml",
		OutputDir: t.TempDir(),
	}
	result, err := apply.DryRun(context.Background(), in)
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if result.Steps[0].Status != "skipped" {
		t.Errorf("step[0] %s: Status = %q; want skipped", result.Steps[0].Name, result.Steps[0].Status)
	}
	if result.Steps[1].Status != "skipped" {
		t.Errorf("step[1] %s: Status = %q; want skipped", result.Steps[1].Name, result.Steps[1].Status)
	}
	if result.Steps[2].Status != "skipped" {
		t.Errorf("step[2] %s: Status = %q; want skipped", result.Steps[2].Name, result.Steps[2].Status)
	}
	if result.Steps[3].Status != "pending" {
		t.Errorf("step[3] %s: Status = %q; want pending", result.Steps[3].Name, result.Steps[3].Status)
	}
}

func TestDryRun_StepOrder(t *testing.T) {
	m := loadTestManifest(t, "central-bank-brl.yaml")
	m.Spec.Node.DataDir = t.TempDir()

	in := apply.ApplyInput{Manifest: m, DryRun: true, OutputFmt: "yaml", OutputDir: t.TempDir()}
	result, _ := apply.DryRun(context.Background(), in)

	expected := orchestrator.CanonicalStepOrder
	if len(result.Steps) != len(expected) {
		t.Fatalf("Steps len = %d; want %d", len(result.Steps), len(expected))
	}
	for i, name := range expected {
		if result.Steps[i].Name != name {
			t.Errorf("Steps[%d].Name = %q; want %q", i, result.Steps[i].Name, name)
		}
	}
}

// ---- helpers ----

// writeTestState writes a ProvisioningState to the canonical path inside dir.
func writeTestState(dir string, state orchestrator.ProvisioningState) error {
	path := filepath.Join(dir, ".provisioning-state.yaml")
	data := "spokeID: " + state.SpokeID + "\nsteps:\n"
	for _, s := range state.Steps {
		data += "  - step: " + s.Step + "\n    status: " + s.Status + "\n    completedAt: " + s.CompletedAt + "\n"
	}
	return os.WriteFile(path, []byte(data), 0o644)
}

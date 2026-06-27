// SPDX-License-Identifier: Apache-2.0

// Package apply — internal tests for run() with injectable engine functions.
// Uses package apply (not apply_test) to access unexported runnerFuncs and run().
package apply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/orchestrator"
)

// internalTestdataDir returns the path to cmd/cbweb3/testdata relative to this package.
func internalTestdataDir(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "..", "cmd", "cbweb3", "testdata")
}

// loadRunManifest loads the canonical test manifest and overrides DataDir to dataDir.
func loadRunManifest(t *testing.T, dataDir string) *manifest.Manifest {
	t.Helper()
	path := filepath.Join(internalTestdataDir(t), "central-bank-brl.yaml")
	m, err := manifest.Load(path)
	if err != nil {
		t.Fatalf("manifest.Load: %v", err)
	}
	m.Spec.Node.DataDir = dataDir
	return m
}

// makeRunInput builds a minimal ApplyInput for run() tests.
func makeRunInput(m *manifest.Manifest, outputDir string) ApplyInput {
	rpcPort := 0
	if m.Spec.Node.RPC != nil {
		rpcPort = m.Spec.Node.RPC.Port
	}
	return ApplyInput{
		Manifest:   m,
		DryRun:     false,
		OutputFmt:  "yaml",
		OutputDir:  outputDir,
		BesuRPCURL: fmt.Sprintf("http://localhost:%d", rpcPort),
	}
}

// writeAllStepsDone writes a state file marking all canonical steps as done.
func writeAllStepsDone(t *testing.T, dataDir string) {
	t.Helper()
	var sb strings.Builder
	sb.WriteString("spokeID: spoke-brl\nsteps:\n")
	for _, name := range orchestrator.CanonicalStepOrder {
		sb.WriteString(fmt.Sprintf("  - step: %s\n    status: done\n    completedAt: 2026-06-27T10:00:00Z\n", name))
	}
	if err := os.WriteFile(filepath.Join(dataDir, ".provisioning-state.yaml"), []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("writeAllStepsDone: %v", err)
	}
}

// writeDoneAndFailed writes a state file with doneCount done steps followed by one failed step.
func writeDoneAndFailed(t *testing.T, dataDir string, doneCount int, failedStep string) {
	t.Helper()
	var sb strings.Builder
	sb.WriteString("spokeID: spoke-brl\nsteps:\n")
	for i := 0; i < doneCount && i < len(orchestrator.CanonicalStepOrder); i++ {
		sb.WriteString(fmt.Sprintf("  - step: %s\n    status: done\n    completedAt: 2026-06-27T10:00:00Z\n", orchestrator.CanonicalStepOrder[i]))
	}
	sb.WriteString(fmt.Sprintf("  - step: %s\n    status: failed\n    completedAt: \n", failedStep))
	if err := os.WriteFile(filepath.Join(dataDir, ".provisioning-state.yaml"), []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("writeDoneAndFailed: %v", err)
	}
}

// T032: Run success path — runFound returns nil → Status "success", Bundle.Path non-empty.
func TestRun_SuccessPath(t *testing.T) {
	dataDir := t.TempDir()
	m := loadRunManifest(t, dataDir)
	outputDir := t.TempDir()

	bundleCalled := false
	fns := runnerFuncs{
		runFound: func(_ context.Context, _ *manifest.Manifest, _ orchestrator.Deps) error {
			return nil
		},
		emitBundle: func(_ context.Context, _ bundle.BundleInput) (*bundle.JoinBundle, error) {
			bundleCalled = true
			return &bundle.JoinBundle{}, nil
		},
	}

	result, err := run(context.Background(), makeRunInput(m, outputDir), fns)

	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Status != "success" {
		t.Errorf("Status = %q; want success", result.Status)
	}
	if result.Bundle == nil || result.Bundle.Path == "" {
		t.Error("Bundle.Path should not be empty on success")
	}
	if len(result.Steps) != 10 {
		t.Errorf("Steps len = %d; want 10", len(result.Steps))
	}
	if !bundleCalled {
		t.Error("emitBundle was not called on success path")
	}
}

// T033: Run failure path — runFound returns error → Status "failed", emitBundle not called.
func TestRun_FailurePath(t *testing.T) {
	dataDir := t.TempDir()
	m := loadRunManifest(t, dataDir)

	fns := runnerFuncs{
		runFound: func(_ context.Context, _ *manifest.Manifest, _ orchestrator.Deps) error {
			return fmt.Errorf("step %s: injected failure", orchestrator.CanonicalStepOrder[2])
		},
		emitBundle: func(_ context.Context, _ bundle.BundleInput) (*bundle.JoinBundle, error) {
			t.Error("emitBundle must not be called when runFound fails")
			return nil, nil
		},
	}

	result, err := run(context.Background(), makeRunInput(m, t.TempDir()), fns)

	if result.Status != "failed" {
		t.Errorf("Status = %q; want failed", result.Status)
	}
	if result.Error == "" {
		t.Error("Error field should be non-empty on failure")
	}
	if err == nil {
		t.Error("expected non-nil error from run on failure path")
	}
	if result.Bundle != nil {
		t.Error("Bundle should be nil on failure")
	}
}

// T034: Run idempotence — all 10 steps pre-populated as done → all steps "skipped", bundle re-emitted.
func TestRun_IdempotenceAllDone(t *testing.T) {
	dataDir := t.TempDir()
	m := loadRunManifest(t, dataDir)
	writeAllStepsDone(t, dataDir)

	bundleCallCount := 0
	fns := runnerFuncs{
		runFound: func(_ context.Context, _ *manifest.Manifest, _ orchestrator.Deps) error {
			return nil
		},
		emitBundle: func(_ context.Context, _ bundle.BundleInput) (*bundle.JoinBundle, error) {
			bundleCallCount++
			return &bundle.JoinBundle{}, nil
		},
	}

	result, err := run(context.Background(), makeRunInput(m, t.TempDir()), fns)

	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Status != "success" {
		t.Errorf("Status = %q; want success", result.Status)
	}
	for _, s := range result.Steps {
		if s.Status != "skipped" {
			t.Errorf("step %s: Status = %q; want skipped (idempotence)", s.Name, s.Status)
		}
	}
	if bundleCallCount != 1 {
		t.Errorf("emitBundle call count = %d; want 1", bundleCallCount)
	}
}

// T035: Run confirms emitBundle is called exactly once after runFound success with correct bundle path.
func TestRun_BundlePathAfterSuccess(t *testing.T) {
	dataDir := t.TempDir()
	m := loadRunManifest(t, dataDir)

	fns := runnerFuncs{
		runFound: func(_ context.Context, _ *manifest.Manifest, _ orchestrator.Deps) error {
			return nil
		},
		emitBundle: func(_ context.Context, _ bundle.BundleInput) (*bundle.JoinBundle, error) {
			return &bundle.JoinBundle{}, nil
		},
	}

	result, err := run(context.Background(), makeRunInput(m, t.TempDir()), fns)

	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.Bundle == nil {
		t.Fatal("Bundle is nil after successful run")
	}
	want := "bundles/spoke-brl.bundle.yaml"
	if result.Bundle.Path != want {
		t.Errorf("Bundle.Path = %q; want %q", result.Bundle.Path, want)
	}
}

// T057: Context cancellation mid-step → last failed step shows "interrupted", overall status "interrupted".
func TestRun_InterruptedStepStatus(t *testing.T) {
	dataDir := t.TempDir()
	m := loadRunManifest(t, dataDir)

	ctx, cancel := context.WithCancel(context.Background())

	fns := runnerFuncs{
		runFound: func(_ context.Context, _ *manifest.Manifest, _ orchestrator.Deps) error {
			// Simulate orchestrator: 2 steps done, step 3 was in progress and marked failed.
			writeDoneAndFailed(t, dataDir, 2, orchestrator.CanonicalStepOrder[2])
			cancel()
			return context.Canceled
		},
		emitBundle: func(_ context.Context, _ bundle.BundleInput) (*bundle.JoinBundle, error) {
			t.Error("emitBundle must not be called on interrupt")
			return nil, nil
		},
	}

	result, _ := run(ctx, makeRunInput(m, t.TempDir()), fns)

	if result.Status != "interrupted" {
		t.Errorf("result.Status = %q; want interrupted", result.Status)
	}
	if result.Steps[0].Status != "skipped" {
		t.Errorf("step[0] %s: Status = %q; want skipped", result.Steps[0].Name, result.Steps[0].Status)
	}
	if result.Steps[1].Status != "skipped" {
		t.Errorf("step[1] %s: Status = %q; want skipped", result.Steps[1].Name, result.Steps[1].Status)
	}
	if result.Steps[2].Status != "interrupted" {
		t.Errorf("step[2] %s: Status = %q; want interrupted", result.Steps[2].Name, result.Steps[2].Status)
	}
	for i := 3; i < len(result.Steps); i++ {
		if result.Steps[i].Status != "pending" {
			t.Errorf("step[%d] %s: Status = %q; want pending", i, result.Steps[i].Name, result.Steps[i].Status)
		}
	}
}

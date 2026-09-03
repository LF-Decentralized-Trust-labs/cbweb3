// SPDX-License-Identifier: Apache-2.0

// Package apply — internal regression tests for the step report. Uses package
// apply (not apply_test) to drive run() with an injected engine.
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

// writeStepsDone writes a state file marking exactly the given steps as done —
// standing in for what a real run records as it executes them.
func writeStepsDone(t *testing.T, dataDir string, steps []string) {
	t.Helper()
	var sb strings.Builder
	sb.WriteString("spokeID: spoke-brl\nsteps:\n")
	for _, name := range steps {
		sb.WriteString(fmt.Sprintf("  - step: %s\n    status: done\n    completedAt: 2026-06-27T10:00:00Z\n", name))
	}
	if err := os.WriteFile(filepath.Join(dataDir, ".provisioning-state.yaml"), []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("writeStepsDone: %v", err)
	}
}

func reportedStepNames(steps []StepResult) []string {
	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s.Name
	}
	return names
}

// A proxy-enabled run executes start-proxy, so its report must list it. It did
// not: the report was built straight from orchestrator.CanonicalStepOrder, which
// omits the step, while only the dry-run path compensated. D14 §6.3 designates
// that report as the deployment's change record, so the omission was an
// audit-trail defect, not a cosmetic one.
func TestRun_ProxyEnabled_ReportListsEveryExecutedStep(t *testing.T) {
	dataDir := t.TempDir()
	m := loadRunManifest(t, dataDir)
	m.Spec.Proxy = "enable"

	executed := orchestrator.PlannedStepOrder(m.Spec.Mode, true)
	fns := runnerFuncs{
		runFound: func(_ context.Context, _ *manifest.Manifest, _ orchestrator.Deps) error {
			writeStepsDone(t, dataDir, executed)
			return nil
		},
		emitBundle: func(_ context.Context, _ bundle.BundleInput) (*bundle.JoinBundle, error) {
			return validEmittedBundleForTest(), nil
		},
	}

	result, err := run(context.Background(), makeRunInput(m, t.TempDir()), fns)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	reported := reportedStepNames(result.Steps)
	if len(reported) != len(executed) {
		t.Fatalf("report lists %d steps but the run executed %d\n executed: %v\n reported: %v",
			len(reported), len(executed), executed, reported)
	}
	for i := range executed {
		if reported[i] != executed[i] {
			t.Errorf("position %d: executed %q, reported %q", i, executed[i], reported[i])
		}
	}

	// The step that used to vanish, spelled out — and reported as run, not pending.
	var proxy *StepResult
	for i := range result.Steps {
		if result.Steps[i].Name == orchestrator.StepStartProxy {
			proxy = &result.Steps[i]
		}
	}
	if proxy == nil {
		t.Fatalf("%q executed but is absent from the report: %v", orchestrator.StepStartProxy, reported)
	}
	if proxy.Status != "skipped" {
		t.Errorf("%q recorded done in state; report Status = %q, want skipped", orchestrator.StepStartProxy, proxy.Status)
	}
}

// The mirror case: with the proxy off, the report must not advertise a step the
// run never executed.
func TestRun_ProxyDisabled_ReportOmitsProxyStep(t *testing.T) {
	dataDir := t.TempDir()
	m := loadRunManifest(t, dataDir)
	m.Spec.Proxy = ""

	fns := runnerFuncs{
		runFound: func(_ context.Context, _ *manifest.Manifest, _ orchestrator.Deps) error {
			writeStepsDone(t, dataDir, orchestrator.CanonicalStepOrder)
			return nil
		},
		emitBundle: func(_ context.Context, _ bundle.BundleInput) (*bundle.JoinBundle, error) {
			return validEmittedBundleForTest(), nil
		},
	}

	result, err := run(context.Background(), makeRunInput(m, t.TempDir()), fns)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, s := range result.Steps {
		if s.Name == orchestrator.StepStartProxy {
			t.Fatalf("proxy disabled but the report lists %q", orchestrator.StepStartProxy)
		}
	}
}

// The plan and the report are the same sequence; a manifest that renders them
// differently is exactly the drift the shared helper exists to prevent.
func TestDryRunPlanMatchesApplyReport_ProxyEnabled(t *testing.T) {
	dataDir := t.TempDir()
	m := loadRunManifest(t, dataDir)
	m.Spec.Proxy = "enable"

	plan, err := DryRun(context.Background(), makeRunInput(m, t.TempDir()))
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}

	fns := runnerFuncs{
		runFound: func(_ context.Context, _ *manifest.Manifest, _ orchestrator.Deps) error { return nil },
		emitBundle: func(_ context.Context, _ bundle.BundleInput) (*bundle.JoinBundle, error) {
			return validEmittedBundleForTest(), nil
		},
	}
	report, err := run(context.Background(), makeRunInput(m, t.TempDir()), fns)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	planned, reported := reportedStepNames(plan.Steps), reportedStepNames(report.Steps)
	if len(planned) != len(reported) {
		t.Fatalf("plan has %d steps, report has %d\n plan:   %v\n report: %v",
			len(planned), len(reported), planned, reported)
	}
	for i := range planned {
		if planned[i] != reported[i] {
			t.Errorf("position %d: plan %q, report %q", i, planned[i], reported[i])
		}
	}
}

// mode:join goes through the same helper, so a proxy-enabled join reports its
// proxy step too — the path the original fix left behind.
func TestPlannedStepOrder_JoinModeReportIncludesProxy(t *testing.T) {
	m := &manifest.Manifest{}
	m.Spec.Mode = "join"
	m.Spec.Proxy = "enable"

	order := plannedStepOrder(m)
	if len(order) != len(orchestrator.CanonicalJoinStepOrder)+1 {
		t.Fatalf("join + proxy: expected %d steps, got %d", len(orchestrator.CanonicalJoinStepOrder)+1, len(order))
	}
	if order[len(order)-1] != orchestrator.StepStartProxy {
		t.Errorf("join + proxy: expected %q last, got %q", orchestrator.StepStartProxy, order[len(order)-1])
	}
}

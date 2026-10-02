// SPDX-License-Identifier: Apache-2.0

// --rebuild has to reach the engine, and the report has to say what it did.
//
// The flag's whole value is that it overrides a satisfied Check for the steps that
// build images. If it is parsed and then dropped anywhere between the CLI and
// orchestrator.Deps, the apply still exits 0 and still reports every step as
// satisfied — the operator sees success and keeps serving the previous binary. That
// silent shape is why this is tested at the boundary rather than only in the engine.
package apply

import (
	"context"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/orchestrator"
)

// TestRebuild_ReachesTheEngineInFoundMode pins the wiring.
func TestRebuild_ReachesTheEngineInFoundMode(t *testing.T) {
	dataDir := t.TempDir()
	m := loadRunManifest(t, dataDir)

	in := makeRunInput(m, t.TempDir())
	in.Rebuild = true

	var got orchestrator.ForcedSteps
	fns := runnerFuncs{
		runFound: func(_ context.Context, _ *manifest.Manifest, deps orchestrator.Deps) (orchestrator.RunOutcome, error) {
			got = deps.Force
			return orchestrator.RunOutcome{}, nil
		},
		emitBundle: func(_ context.Context, _ bundle.BundleInput) (*bundle.JoinBundle, error) {
			return validEmittedBundleForTest(), nil
		},
	}

	if _, err := run(context.Background(), in, fns); err != nil {
		t.Fatalf("run: %v", err)
	}

	want := orchestrator.RebuildForcedSteps("found")
	if len(got) != len(want) {
		t.Fatalf("engine received Force = %v, want %v", got, want)
	}
	for name := range want {
		if !got[name] {
			t.Errorf("--rebuild did not force %q; its Check would resolve the step and the "+
				"rebuild would not happen", name)
		}
	}
}

// TestWithoutRebuild_NothingIsForced is the half that keeps the flag meaningful: an
// ordinary apply must still honour every Check, or --rebuild becomes the default and
// every deploy pays for a full image build.
func TestWithoutRebuild_NothingIsForced(t *testing.T) {
	dataDir := t.TempDir()
	m := loadRunManifest(t, dataDir)

	var got orchestrator.ForcedSteps
	fns := runnerFuncs{
		runFound: func(_ context.Context, _ *manifest.Manifest, deps orchestrator.Deps) (orchestrator.RunOutcome, error) {
			got = deps.Force
			return orchestrator.RunOutcome{}, nil
		},
		emitBundle: func(_ context.Context, _ bundle.BundleInput) (*bundle.JoinBundle, error) {
			return validEmittedBundleForTest(), nil
		},
	}

	if _, err := run(context.Background(), makeRunInput(m, t.TempDir()), fns); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("an apply without --rebuild forced %v; every Check must stand", got)
	}
}

// TestRebuild_ForcedStepIsReportedAsExecuted joins the two halves of this branch: the
// step ran only because it was forced, and the report can now say so — the outcome
// comes from the engine rather than from a state diff, which is what a forced re-run
// of an already-done step defeats.
func TestRebuild_ForcedStepIsReportedAsExecuted(t *testing.T) {
	dataDir := t.TempDir()
	m := loadRunManifest(t, dataDir)

	in := makeRunInput(m, t.TempDir())
	in.Rebuild = true

	fns := runnerFuncs{
		runFound: func(_ context.Context, _ *manifest.Manifest, deps orchestrator.Deps) (orchestrator.RunOutcome, error) {
			// Everything was already done before this run; the forced step is the
			// only one the engine executed.
			writeStepsDone(t, dataDir, orchestrator.CanonicalStepOrder)
			return ranOutcome(orchestrator.StepStartCBBackend), nil
		},
		emitBundle: func(_ context.Context, _ bundle.BundleInput) (*bundle.JoinBundle, error) {
			return validEmittedBundleForTest(), nil
		},
	}

	result, err := run(context.Background(), in, fns)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	for _, sr := range result.Steps {
		switch sr.Name {
		case orchestrator.StepStartCBBackend:
			if sr.Status != "executed" {
				t.Errorf("%s: status = %q, want executed — it was rebuilt by this run",
					sr.Name, sr.Status)
			}
		default:
			if sr.Status == "executed" {
				t.Errorf("%s: status = executed, but the engine did not run it", sr.Name)
			}
		}
	}
}

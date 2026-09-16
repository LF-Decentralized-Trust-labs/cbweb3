// SPDX-License-Identifier: Apache-2.0

// --rebuild must make a step run even though its Check says there is nothing to do.
//
// Every Check in this engine that guards a build step is a liveness probe: the
// backend step asks whether /healthz answers, the frontend step whether each portal
// serves a page. Neither question has anything to do with the source tree. Edit a Go
// service or a portal, re-apply, and the container is still healthy — so Check
// answers true, Run is never called, and the `docker compose up --build` inside it
// never happens. The apply reports the step as satisfied and the stack keeps serving
// the previous binary.
//
// The workaround operators reached for was deleting the step's line from
// .provisioning-state.yaml. That does nothing here — this engine calls Check on
// every step regardless of persisted state — which is how it also produced a false
// "executed" in the report (see engine/apply/step_report_status_test.go).
package orchestrator

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestForce_RunsAStepWhoseCheckIsSatisfied is the mechanism.
func TestForce_RunsAStepWhoseCheckIsSatisfied(t *testing.T) {
	dataDir := t.TempDir()
	writeGenesisJSON(t, dataDir)

	forced := &mockStep{name: "start-cb-backend", checkVal: true}
	untouched := &mockStep{name: "start-besu", checkVal: true}

	deps := testDeps()
	deps.Force = ForcedSteps{"start-cb-backend": true}

	var buf bytes.Buffer
	outcome, err := runFoundWithSteps(context.Background(), testManifest(dataDir), deps,
		&buf, []Step{forced, untouched})
	if err != nil {
		t.Fatalf("runFoundWithSteps: %v", err)
	}

	if forced.runCalled != 1 {
		t.Errorf("forced step ran %d times, want 1: its Check reported satisfied, which under "+
			"--rebuild is exactly the answer that must be overridden — a healthy container is "+
			"not evidence that it is running the current source", forced.runCalled)
	}
	if untouched.runCalled != 0 {
		t.Errorf("unforced step ran %d times, want 0: --rebuild targets the steps that build "+
			"images, not the whole pipeline — re-running start-besu is not what was asked for",
			untouched.runCalled)
	}

	// The report must agree, which it only can because the engine now returns what it ran.
	if !outcome.Ran["start-cb-backend"] {
		t.Error("the forced step is absent from the outcome, so the report would call the " +
			"rebuild skipped")
	}
	if outcome.Ran["start-besu"] {
		t.Error("an unforced, check-satisfied step is recorded as ran")
	}
}

// TestForce_LogsWhyTheCheckWasOverridden: the operator has to be able to tell a
// forced run from an ordinary one, or the next person reading the log sees a step
// re-running with no explanation.
func TestForce_LogsWhyTheCheckWasOverridden(t *testing.T) {
	dataDir := t.TempDir()
	writeGenesisJSON(t, dataDir)

	deps := testDeps()
	deps.Force = ForcedSteps{"start-cb-frontend": true}

	var buf bytes.Buffer
	if _, err := runFoundWithSteps(context.Background(), testManifest(dataDir), deps, &buf,
		[]Step{&mockStep{name: "start-cb-frontend", checkVal: true}}); err != nil {
		t.Fatalf("runFoundWithSteps: %v", err)
	}

	if !strings.Contains(buf.String(), "rebuild") {
		t.Errorf("the log does not mention why start-cb-frontend ran despite a satisfied "+
			"check:\n%s", buf.String())
	}
}

// TestForce_JoinModeHonoursIt too: a commercial bank's backend and portal are built
// from the same source tree and go stale the same way.
func TestForce_JoinModeHonoursIt(t *testing.T) {
	dataDir := t.TempDir()
	m := testJoinManifest(dataDir)

	forced := &mockStep{name: StepStartBackend, checkVal: true}

	deps := testJoinDeps()
	deps.Force = ForcedSteps{StepStartBackend: true}

	var buf bytes.Buffer
	if _, err := runJoinWithSteps(context.Background(), m, testJoinBundle(), deps, &buf,
		[]Step{forced}); err != nil {
		t.Fatalf("runJoinWithSteps: %v", err)
	}
	if forced.runCalled != 1 {
		t.Errorf("%s ran %d times under --rebuild, want 1", StepStartBackend, forced.runCalled)
	}
}

// TestRebuildForcedSteps_NamesOnlyRealSteps is the guard on the guard, and the one
// that earns its keep: a forced-step set is a map of strings matched against step
// names, so a typo or a renamed constant silently forces nothing. --rebuild would
// then appear to work, exit 0, and ship the stale binary it was invoked to replace.
func TestRebuildForcedSteps_NamesOnlyRealSteps(t *testing.T) {
	for _, mode := range []string{"found", "join"} {
		t.Run(mode, func(t *testing.T) {
			forced := RebuildForcedSteps(mode)
			if len(forced) == 0 {
				t.Fatalf("mode %s forces no steps, so --rebuild is a no-op there", mode)
			}

			known := make(map[string]bool)
			for _, n := range PlannedStepOrder(mode, true) {
				known[n] = true
			}
			for name := range forced {
				if !known[name] {
					t.Errorf("%q is forced in mode %s but is not a step that mode runs — "+
						"--rebuild would silently do nothing for it; known steps: %v",
						name, mode, PlannedStepOrder(mode, true))
				}
			}
		})
	}
}

// An unknown mode must force nothing rather than guess. observe rebuilds its images
// on every run already (it has no Check to satisfy), so it needs no forcing.
func TestRebuildForcedSteps_UnknownModeForcesNothing(t *testing.T) {
	for _, mode := range []string{"observe", "", "found-hub"} {
		if got := RebuildForcedSteps(mode); len(got) != 0 {
			t.Errorf("mode %q: forces %v, want nothing", mode, got)
		}
	}
}

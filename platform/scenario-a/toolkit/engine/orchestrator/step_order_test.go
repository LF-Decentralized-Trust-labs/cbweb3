// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"io"
	"testing"
)

// stepNames extracts the execution sequence the engine actually built.
func stepNames(steps []Step) []string {
	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s.Name()
	}
	return names
}

func assertSameOrder(t *testing.T, label string, want, got []string) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("%s: declared %d steps, engine builds %d\n declared: %v\n executed: %v",
			label, len(want), len(got), want, got)
	}
	for i := range want {
		if want[i] != got[i] {
			t.Errorf("%s: position %d declares %q but the engine runs %q\n declared: %v\n executed: %v",
				label, i, want[i], got[i], want, got)
		}
	}
}

// The declared order and the executed order are the same thing rendered twice —
// the apply report is built from the declared one, so any drift means the archived
// report lists a sequence the run did not follow. register-relay is the step this
// once got wrong: the constant placed it right after deploy-htlc while the engine
// runs it last of the provisioning steps, once the CB coordinator endpoints exist.
func TestCanonicalStepOrderMatchesExecution(t *testing.T) {
	m := testManifest(t.TempDir())
	steps := buildSteps(m, testDeps(), m.Spec.Node.DataDir, ProvisioningState{})
	assertSameOrder(t, "mode:found", CanonicalStepOrder, stepNames(steps))
}

// Same contract for mode:join — the join report is rendered from
// CanonicalJoinStepOrder, so it must match what buildJoinSteps assembles.
func TestCanonicalJoinStepOrderMatchesExecution(t *testing.T) {
	dataDir := t.TempDir()
	m := testJoinManifest(dataDir)
	steps := buildJoinSteps(m, testJoinBundle(), testJoinDeps(), dataDir, io.Discard)
	assertSameOrder(t, "mode:join", CanonicalJoinStepOrder, stepNames(steps))
}

// A proxy-enabled join executes the proxy step too — the path the original
// dry-run-only compensation left unreported.
func TestJoinPlannedStepOrderCoversProxyStep(t *testing.T) {
	dataDir := t.TempDir()
	m := testJoinManifest(dataDir)
	m.Spec.Proxy = "enable"
	steps := buildJoinSteps(m, testJoinBundle(), testJoinDeps(), dataDir, io.Discard)
	assertSameOrder(t, "mode:join + proxy", PlannedStepOrder("join", true), stepNames(steps))
}

// A proxy-enabled run executes one more step than the canonical order declares.
// PlannedStepOrder is what closes that gap; this asserts it closes it exactly.
func TestPlannedStepOrderCoversProxyStep(t *testing.T) {
	m := testManifest(t.TempDir())
	m.Spec.Proxy = "enable"

	steps := buildSteps(m, testDeps(), m.Spec.Node.DataDir, ProvisioningState{})
	assertSameOrder(t, "mode:found + proxy", PlannedStepOrder("found", true), stepNames(steps))

	// The canonical slice alone would under-report by exactly the proxy step.
	if len(CanonicalStepOrder) != len(steps)-1 {
		t.Errorf("expected the proxy run to execute exactly one step beyond CanonicalStepOrder, got %d vs %d",
			len(steps), len(CanonicalStepOrder))
	}
}

// Without the proxy, the planned order must not invent a step the run never executes.
func TestPlannedStepOrderWithoutProxy(t *testing.T) {
	m := testManifest(t.TempDir())
	steps := buildSteps(m, testDeps(), m.Spec.Node.DataDir, ProvisioningState{})
	assertSameOrder(t, "mode:found, proxy disabled", PlannedStepOrder("found", false), stepNames(steps))

	for _, name := range PlannedStepOrder("found", false) {
		if name == StepStartProxy {
			t.Fatalf("proxy disabled but %q is in the planned order", StepStartProxy)
		}
	}
}

// PlannedStepOrder hands out a copy: a caller appending to the returned slice must
// not corrupt the package-level order for every subsequent run in the process.
func TestPlannedStepOrderReturnsCopy(t *testing.T) {
	before := len(CanonicalStepOrder)
	order := PlannedStepOrder("found", false)
	order = append(order, "mutation-canary")
	if len(CanonicalStepOrder) != before {
		t.Fatalf("CanonicalStepOrder grew from %d to %d — PlannedStepOrder returned the shared slice",
			before, len(CanonicalStepOrder))
	}
	if CanonicalStepOrder[len(CanonicalStepOrder)-1] == "mutation-canary" {
		t.Fatal("append through PlannedStepOrder overwrote CanonicalStepOrder")
	}
	_ = order
}

// mode:join dispatches to the join order, and picks up the proxy step the same way.
func TestPlannedStepOrderJoinMode(t *testing.T) {
	got := PlannedStepOrder("join", false)
	assertSameOrder(t, "mode:join", CanonicalJoinStepOrder, got)

	withProxy := PlannedStepOrder("join", true)
	if len(withProxy) != len(CanonicalJoinStepOrder)+1 {
		t.Fatalf("join + proxy: expected %d steps, got %d", len(CanonicalJoinStepOrder)+1, len(withProxy))
	}
	if withProxy[len(withProxy)-1] != StepStartProxy {
		t.Errorf("join + proxy: expected %q last, got %q", StepStartProxy, withProxy[len(withProxy)-1])
	}
}

// An empty mode is mode:found (see apply.run's `case "found", ""`).
func TestPlannedStepOrderEmptyModeIsFound(t *testing.T) {
	assertSameOrder(t, "empty mode", CanonicalStepOrder, PlannedStepOrder("", false))
}

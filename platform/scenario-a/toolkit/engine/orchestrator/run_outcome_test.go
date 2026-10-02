// SPDX-License-Identifier: Apache-2.0

// The engine must report which steps it actually RAN, because nothing else can.
//
// The state file records "done" for a step whose Run succeeded AND for a step the
// Check found already satisfied — deliberately, so a stale "failed" converges on an
// idempotent re-run. Two steps then branch on that exact value
// (step_register_nodes.go:46, step_register_paladin_node.go:68), so the persisted
// vocabulary cannot be widened to carry the distinction without changing when those
// steps re-execute.
//
// The apply report used to infer "executed" from a state snapshot taken before the
// run: a step absent from it and done after it was labelled executed. That is a
// different question from "did this run do the work", and the answer diverges in the
// case below — which is exactly the case that cost an afternoon: a step deleted from
// the state file to force a rebuild reported `executed` while Check short-circuited
// and `docker compose --build` never ran.
package orchestrator

import (
	"bytes"
	"context"
	"testing"
)

// TestRunOutcome_SeparatesRanFromCheckSatisfied is the property the report needs.
//
// Both steps end the run recorded as done in state; only one of them ran. The
// outcome must tell them apart, and it must do so for a step that had NO prior
// state entry — the combination the old inference got wrong.
func TestRunOutcome_SeparatesRanFromCheckSatisfied(t *testing.T) {
	dataDir := t.TempDir()
	writeGenesisJSON(t, dataDir)

	alreadyDone := &mockStep{name: "check-satisfied", checkVal: true}
	needsRunning := &mockStep{name: "really-ran", checkVal: false}

	var buf bytes.Buffer
	outcome, err := runFoundWithSteps(context.Background(), testManifest(dataDir), testDeps(),
		&buf, []Step{alreadyDone, needsRunning})
	if err != nil {
		t.Fatalf("runFoundWithSteps: %v", err)
	}

	// The mock's own counter is the ground truth the outcome has to agree with.
	if alreadyDone.runCalled != 0 {
		t.Fatalf("precondition: the check-satisfied step ran %d times", alreadyDone.runCalled)
	}
	if needsRunning.runCalled != 1 {
		t.Fatalf("precondition: the other step ran %d times, want 1", needsRunning.runCalled)
	}

	if outcome.Ran["check-satisfied"] {
		t.Error(`outcome claims "check-satisfied" ran; Check answered and Run was never called. ` +
			`This is the case that makes the apply report say "executed" for work nobody did.`)
	}
	if !outcome.Ran["really-ran"] {
		t.Error(`outcome omits "really-ran", whose Run was invoked — a report built on this ` +
			`would call executed work skipped, which is the opposite error`)
	}
}

// TestRunOutcome_IsReportedEvenWhenAStepFails: the run aborts, but the steps that
// already ran still ran. A report that loses them because of a later failure
// describes the wrong run.
func TestRunOutcome_IsReportedEvenWhenAStepFails(t *testing.T) {
	dataDir := t.TempDir()
	writeGenesisJSON(t, dataDir)

	ok := &mockStep{name: "ran-fine", checkVal: false}
	boom := &mockStep{name: "exploded", checkVal: false, runErr: context.DeadlineExceeded}

	var buf bytes.Buffer
	outcome, err := runFoundWithSteps(context.Background(), testManifest(dataDir), testDeps(),
		&buf, []Step{ok, boom})
	if err == nil {
		t.Fatal("expected the failing step to abort the run")
	}
	if !outcome.Ran["ran-fine"] {
		t.Error(`the step that ran before the failure is missing from the outcome`)
	}
	if outcome.Ran["exploded"] {
		t.Error(`a step whose Run returned an error is recorded as ran; the report would ` +
			`then show it as executed rather than failed`)
	}
}

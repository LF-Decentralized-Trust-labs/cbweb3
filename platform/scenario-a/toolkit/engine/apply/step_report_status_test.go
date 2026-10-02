// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/orchestrator"
)

// The report used to call every completed step "skipped", whether this run did the
// work or found it already done. On a first apply that produced a report in which
// all ten steps read `status: skipped` while each carried a fresh completedAt —
// contradicting itself, and telling the operator nothing about what just happened.
//
// The first fix inferred the distinction by diffing the final state against a
// snapshot taken before the engine ran. That answers "is it done now that was not
// done before", which is a near-miss for "did this run do the work" — and the two
// disagree in the one case an operator most needs the report to be right about (see
// TestStepReportCallsACheckSatisfiedStepSkipped). The engine now returns the set of
// steps whose Run it actually called, and that set is what these tests pass in.

func doneState(names ...string) orchestrator.ProvisioningState {
	st := orchestrator.ProvisioningState{}
	for _, n := range names {
		st.Steps = append(st.Steps, orchestrator.StepState{
			Step: n, Status: "done", CompletedAt: "2026-09-02T18:47:01Z",
		})
	}
	return st
}

// ranSet is the engine's answer, in the shape buildStepResults consumes.
func ranSet(names ...string) map[string]bool {
	ran := make(map[string]bool, len(names))
	for _, n := range names {
		ran[n] = true
	}
	return ran
}

// TestStepReportDistinguishesExecutedFromSkipped is the whole point: same final
// state, different report, decided by what the engine actually ran.
func TestStepReportDistinguishesExecutedFromSkipped(t *testing.T) {
	order := []string{"write-genesis", "start-besu", "deploy-contracts"}
	state := doneState("write-genesis", "start-besu", "deploy-contracts")

	// The engine's Check found "write-genesis" already satisfied and ran the other
	// two. All three end the run marked done — that is why state alone cannot
	// answer this and the engine has to.
	results := buildStepResults(order, state, ranSet("start-besu", "deploy-contracts"))

	want := map[string]string{
		"write-genesis":    "skipped",
		"start-besu":       "executed",
		"deploy-contracts": "executed",
	}
	for _, sr := range results {
		if got := sr.Status; got != want[sr.Name] {
			t.Errorf("%s: status = %q, want %q", sr.Name, got, want[sr.Name])
		}
		if sr.CompletedAt == "" {
			t.Errorf("%s: completedAt is empty; a finished step must carry its timestamp", sr.Name)
		}
	}
}

// TestStepReportOnAFirstRunClaimsNothingWasSkipped covers the case that exposed the
// original bug: a run that started from nothing did every step, and a report calling
// them skipped while stamping each with a fresh completedAt contradicts itself.
func TestStepReportOnAFirstRunClaimsNothingWasSkipped(t *testing.T) {
	order := []string{"write-genesis", "start-besu"}
	results := buildStepResults(order, doneState(order...), ranSet(order...))

	for _, sr := range results {
		if sr.Status == "skipped" {
			t.Errorf("%s reported as skipped on a run that executed it", sr.Name)
		}
		if sr.Status != "executed" {
			t.Errorf("%s: status = %q, want %q", sr.Name, sr.Status, "executed")
		}
	}
}

// TestStepReportCallsACheckSatisfiedStepSkipped is the regression the pre-run
// snapshot could not catch, and the reason the engine now reports its own outcome.
//
// The scenario is ordinary: an operator deletes a step from
// .provisioning-state.yaml to force it to run again — the documented way to make
// start-besu rebuild. The step is then absent from any pre-run snapshot, so the old
// inference labelled it "executed" the moment it appeared as done afterwards. But
// the engine's Check looks at the world, not at the file: the container is up, so
// Check answers true, Run is never called, and `docker compose --build` never
// happens. The report said the rebuild ran. It had not.
func TestStepReportCallsACheckSatisfiedStepSkipped(t *testing.T) {
	order := []string{"start-besu"}

	// No prior state entry (it was deleted), done afterwards, and the engine ran
	// nothing — exactly the combination that used to read "executed".
	results := buildStepResults(order, doneState("start-besu"), ranSet())

	if results[0].Status != "skipped" {
		t.Errorf("start-besu: status = %q, want skipped — Check resolved the step and Run was "+
			"never called, so reporting it as executed tells the operator a rebuild happened "+
			"when it did not", results[0].Status)
	}
}

// Failed and never-reached steps must keep their meaning: the new distinction only
// applies to work that finished.
func TestStepReportKeepsFailedAndPending(t *testing.T) {
	order := []string{"write-genesis", "start-besu", "deploy-contracts"}
	state := orchestrator.ProvisioningState{Steps: []orchestrator.StepState{
		{Step: "write-genesis", Status: "done", CompletedAt: "2026-09-02T18:47:01Z"},
		{Step: "start-besu", Status: "failed"},
	}}

	results := buildStepResults(order, state, ranSet())

	want := map[string]string{
		"write-genesis":    "skipped",
		"start-besu":       "failed",
		"deploy-contracts": "pending",
	}
	for _, sr := range results {
		if sr.Status != want[sr.Name] {
			t.Errorf("%s: status = %q, want %q", sr.Name, sr.Status, want[sr.Name])
		}
	}
}

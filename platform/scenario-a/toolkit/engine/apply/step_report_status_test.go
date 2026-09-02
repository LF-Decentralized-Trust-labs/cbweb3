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
// What separates the two is the state that existed BEFORE the engine ran, which is
// why the pre-run set of finished steps is now passed in rather than inferred from
// timestamps: comparing completedAt against a run start would depend on clock
// resolution and on the state file and the report agreeing about time.

func doneState(names ...string) orchestrator.ProvisioningState {
	st := orchestrator.ProvisioningState{}
	for _, n := range names {
		st.Steps = append(st.Steps, orchestrator.StepState{
			Step: n, Status: "done", CompletedAt: "2026-09-02T18:47:01Z",
		})
	}
	return st
}

// TestStepReportDistinguishesExecutedFromSkipped is the whole point: same final
// state, different report, decided by what was already done beforehand.
func TestStepReportDistinguishesExecutedFromSkipped(t *testing.T) {
	order := []string{"write-genesis", "start-besu", "deploy-contracts"}
	state := doneState("write-genesis", "start-besu", "deploy-contracts")

	// "write-genesis" was already finished when this run started; the other two
	// were not, so this run did them.
	results := buildStepResults(order, state, map[string]bool{"write-genesis": true})

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
// bug: nothing existed before, so nothing can honestly be reported as skipped.
func TestStepReportOnAFirstRunClaimsNothingWasSkipped(t *testing.T) {
	order := []string{"write-genesis", "start-besu"}
	results := buildStepResults(order, doneState("write-genesis", "start-besu"), nil)

	for _, sr := range results {
		if sr.Status == "skipped" {
			t.Errorf("%s reported as skipped on a run that had no prior state", sr.Name)
		}
		if sr.Status != "executed" {
			t.Errorf("%s: status = %q, want %q", sr.Name, sr.Status, "executed")
		}
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

	results := buildStepResults(order, state, map[string]bool{"write-genesis": true})

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

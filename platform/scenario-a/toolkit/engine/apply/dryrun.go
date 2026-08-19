// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/orchestrator"
)

// DryRun reads current provisioning state and returns a plan report.
// No side effects: no files written, no engine called, no Besu accessed.
func DryRun(ctx context.Context, in ApplyInput) (ApplyResult, error) {
	m := in.Manifest

	// observe has no on-chain node/state file; report its linear plan directly.
	if m.Spec.Mode == "observe" {
		in.DryRun = true
		return runObserveMode(ctx, in)
	}

	result := ApplyResult{
		Spoke:  m.Spec.Spoke.ID,
		Mode:   m.Spec.Mode,
		DryRun: true,
		Status: "dry-run",
	}

	// Read state file — missing file means all steps pending.
	state, _ := orchestrator.LoadState(m.Spec.Node.DataDir)

	// Same source the apply report uses — mode dispatch and the soft-appended
	// reverse-proxy step both live in PlannedStepOrder, so plan and report cannot
	// drift apart.
	stepOrder := plannedStepOrder(m)

	steps := make([]StepResult, len(stepOrder))
	for i, stepName := range stepOrder {
		status := "pending"
		for _, s := range state.Steps {
			if s.Step == stepName && s.Status == "done" {
				status = "skipped"
				break
			}
		}
		steps[i] = StepResult{Name: stepName, Status: status}
	}
	result.Steps = steps
	return result, nil
}

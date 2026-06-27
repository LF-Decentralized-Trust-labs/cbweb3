// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/orchestrator"
)

// DryRun reads current provisioning state and returns a plan report.
// No side effects: no files written, no engine called, no Besu accessed.
func DryRun(_ context.Context, in ApplyInput) (ApplyResult, error) {
	m := in.Manifest

	result := ApplyResult{
		Spoke:  m.Spec.Spoke.ID,
		Mode:   m.Spec.Mode,
		DryRun: true,
		Status: "dry-run",
	}

	// Read state file — missing file means all steps pending.
	state, _ := orchestrator.LoadState(m.Spec.Node.DataDir)

	steps := make([]StepResult, len(orchestrator.CanonicalStepOrder))
	for i, stepName := range orchestrator.CanonicalStepOrder {
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

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

	// Select the step order for the manifest's mode.
	stepOrder := orchestrator.CanonicalStepOrder
	if m.Spec.Mode == "join" {
		stepOrder = orchestrator.CanonicalJoinStepOrder
	}
	// The reverse-proxy step is soft-appended by the orchestrator ONLY when
	// spec.proxy == "enable" (see buildSteps / buildJoinSteps). Mirror that here so
	// the plan reflects it. Copy the shared canonical slice before appending — never
	// mutate the package-level order.
	if m.Spec.Proxy == "enable" {
		order := make([]string, len(stepOrder), len(stepOrder)+1)
		copy(order, stepOrder)
		stepOrder = append(order, orchestrator.StepStartProxy)
	}

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

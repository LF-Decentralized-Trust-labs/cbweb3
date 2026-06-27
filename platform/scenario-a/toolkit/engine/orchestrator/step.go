// SPDX-License-Identifier: Apache-2.0

package orchestrator

import "context"

// Step is the unit of work in the orchestration engine.
// Each step is idempotent: Check returns true if the step is already complete.
type Step interface {
	// Name returns the canonical step name constant (e.g. StepDeployContracts).
	Name() string

	// Check returns true if this step has already been completed successfully.
	// Check must not have side effects and must not modify external state.
	Check(ctx context.Context) (bool, error)

	// Run executes the step. Called only when Check returns false.
	// A non-nil error causes the orchestrator to halt and mark the step as "failed".
	Run(ctx context.Context) error
}

// Canonical step name constants — used as keys in ProvisioningState and log events.
const (
	StepDeployContracts = "deploy-contracts"
	StepGenTLS          = "gen-tls"
	StepRenderConfigs   = "render-configs"
	StepRegisterNodes   = "register-nodes"
	StepStartPaladin    = "start-paladin"
	StepCreateZetoToken = "create-zeto-token"
	StepCreatePente     = "create-pente-context"
	StepDeployFXAPente  = "deploy-fxa-pente"
	StepOnboardRegistry = "onboard-registry"
	StepRegisterRelay   = "register-relay"
)

// CanonicalStepOrder is the definitive execution sequence for mode:found.
var CanonicalStepOrder = []string{
	StepDeployContracts,
	StepGenTLS,
	StepRenderConfigs,
	StepRegisterNodes,
	StepStartPaladin,
	StepCreateZetoToken,
	StepCreatePente,
	StepDeployFXAPente,
	StepOnboardRegistry,
	StepRegisterRelay,
}

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
	StepStartBesu       = "start-besu"
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
	// CB operational stack (feature 034 US1): dedicated infra + Keycloak + backend.
	StepRenderCBEnv       = "render-cb-env"
	StepStartCBInfra      = "start-cb-infra"
	StepProvisionKeycloak = "provision-keycloak"
	StepStartCBBackend    = "start-cb-backend"
)

// CanonicalStepOrder is the definitive execution sequence for mode:found.
// found is CB-only: it deploys spoke-level contracts and stands up the central
// bank's Besu + Paladin, but does NOT create the bilateral Pente context or
// deploy FXAgreement-in-Pente — those are per-relationship and are created at
// join time (feature 033), when a commercial bank's Paladin node exists.
var CanonicalStepOrder = []string{
	StepStartBesu,
	StepDeployContracts,
	StepGenTLS,
	StepRenderConfigs,
	StepRegisterNodes,
	StepStartPaladin,
	StepCreateZetoToken,
	StepOnboardRegistry,
	StepRegisterRelay,
	StepRenderCBEnv,
	StepStartCBInfra,
	StepProvisionKeycloak,
	StepStartCBBackend,
}

// Canonical step name constants for mode:join. Kept in a namespace distinct from
// mode:found so both modes can share a SPOKE_DATA_DIR state file without collision.
const (
	StepWriteGenesis    = "write-genesis"
	StepStartBesuJoin   = "start-besu-join"
	StepWaitSync        = "wait-sync"
	StepVoteQBFT        = "vote-qbft"
	StepGenCSR          = "gen-csr"
	StepRequestCert     = "request-cert"
	StepReceiveCert     = "receive-cert"
	StepProofPossession = "proof-of-possession"
	// Dynamic Paladin node bring-up for the joining commercial bank (feature 033 US2).
	StepGenTLSJoin          = "gen-tls-join"
	StepRenderConfigJoin    = "render-config-join"
	StepStartPaladinJoin    = "start-paladin-join"
	StepRegisterPaladinNode = "register-paladin-node"
	// Bilateral Pente context + FXAgreement deploy for the CB↔bank relationship (US3).
	StepCreatePenteJoin = "create-pente-context"
	StepDeployFXAJoin   = "deploy-fxa-pente"
	StepStartBackend    = "start-backend"
)

// CanonicalJoinStepOrder is the definitive execution sequence for mode:join.
// After the bank is registered on-chain (proof-of-possession), its Paladin node
// is brought up and registered dynamically (US2), then the backend starts.
var CanonicalJoinStepOrder = []string{
	StepWriteGenesis,
	StepStartBesuJoin,
	StepWaitSync,
	StepVoteQBFT,
	StepGenCSR,
	StepRequestCert,
	StepReceiveCert,
	StepProofPossession,
	StepGenTLSJoin,
	StepRenderConfigJoin,
	StepStartPaladinJoin,
	StepRegisterPaladinNode,
	StepCreatePenteJoin,
	StepDeployFXAJoin,
	StepStartBackend,
}

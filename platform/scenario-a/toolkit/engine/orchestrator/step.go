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
	// Besu-layer settlement contracts (Scenario A): fCeBM ERC-20 + HTLC. Deployed
	// after onboard-registry because HTLC's constructor takes the IdentityRegistry
	// (PARTICIPANT_REGISTRY_ADDRESS) that step produces.
	StepDeployFiatToken = "deploy-fiat-token"
	StepDeployHTLC      = "deploy-htlc"
	StepRegisterRelay   = "register-relay"
	// CB operational stack (feature 034 US1): dedicated infra + Keycloak + backend.
	StepRenderCBEnv       = "render-cb-env"
	StepStartCBInfra      = "start-cb-infra"
	StepProvisionKeycloak = "provision-keycloak"
	StepStartCBBackend    = "start-cb-backend"
	StepStartCBFrontend   = "start-cb-frontend"
	// Per-entity launcher (distributed A/B entry point). Runs in both modes.
	StepStartLauncher = "start-launcher"
	// Per-host reverse proxy (Caddy): one :80 entrypoint routing portals + api by path.
	StepStartProxy = "start-proxy"
)

// CanonicalStepOrder is the definitive execution sequence for mode:found.
// found is CB-only: it deploys spoke-level contracts and stands up the central
// bank's Besu + Paladin, but does NOT create the bilateral Pente context or
// deploy FXAgreement-in-Pente — those are per-relationship and are created at
// join time (feature 033), when a commercial bank's Paladin node exists.
//
// This slice must stay in lockstep with the order buildSteps appends the steps
// in — it is what the apply report and the dry-run plan are rendered from, and a
// report that lists steps in an order the run did not follow is an audit-trail
// defect. StepRegisterRelay in particular runs LAST of the provisioning steps
// (after the frontend, once the CB coordinator endpoints exist), not right after
// deploy-htlc. TestCanonicalStepOrderMatchesExecution locks the two together.
//
// It never includes StepStartProxy: that step is appended only when
// spec.proxy == "enable". Build the order a run will actually execute with
// PlannedStepOrder rather than reading this slice directly.
var CanonicalStepOrder = []string{
	StepStartBesu,
	StepDeployContracts,
	StepGenTLS,
	StepRenderConfigs,
	StepRegisterNodes,
	StepStartPaladin,
	StepCreateZetoToken,
	StepOnboardRegistry,
	StepDeployFiatToken,
	StepDeployHTLC,
	StepRenderCBEnv,
	StepStartCBInfra,
	StepProvisionKeycloak,
	StepStartCBBackend,
	StepStartCBFrontend,
	StepRegisterRelay,
	StepStartLauncher,
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
	// Commercial bank operational stack (feature 034 US2): dedicated infra +
	// per-entity Keycloak + the 4 backend services, in commercial-bank mode.
	StepRenderBankEnv         = "render-bank-env"
	StepStartBankInfra        = "start-bank-infra"
	StepProvisionBankKeycloak = "provision-bank-keycloak"
	StepStartBackend          = "start-backend"
	StepStartBankFrontend     = "start-bank-frontend"
)

// CanonicalJoinStepOrder is the definitive execution sequence for mode:join.
// After the bank is registered on-chain (proof-of-possession), its Paladin node
// is brought up and registered dynamically (US2), then the backend starts.
// The governance-gated identity steps run LAST, off the critical path, so the bank
// fully provisions regardless of approval timing:
//   - proof-of-possession registers the bank as a participant in IdentityRegistry,
//     which is onlyRole(GOVERNANCE_ROLE) — performed by the CB on KYC approval
//     (approve-kyc → setParticipant), not by the bank;
//   - gen-csr → request-cert → receive-cert acquire the CB-signed PKI cert, a
//     runtime identity credential issued only after a governance KYC approval.
//
// None of these are consumed by any provisioning step (Paladin uses a self-signed
// transport cert, the node self-registers via registerIdentity, the backend mounts
// no bank CA). The deferred-tail steps are soft (see RunJoin):
//   - create-pente-context / deploy-fxa-pente: the bilateral CB↔bank Pente group
//     and FXAgreement. The cross-node Paladin transport these need now works (the
//     gen-tls cert CN was fixed to equal the node name — see E2E-STATUS.md), and
//     create-pente-context opens with a peer-readiness gate so it no longer races
//     the soft tail (it waits for the remote Paladin transport before creating the
//     group). They do not gate provisioning — the bank's backend boots without the
//     bilateral FXAgreement.
//   - proof-of-possession / receive-cert: governance-gated identity (see above).
var CanonicalJoinStepOrder = []string{
	StepWriteGenesis,
	StepStartBesuJoin,
	StepWaitSync,
	StepGenTLSJoin,
	StepRenderConfigJoin,
	StepStartPaladinJoin,
	StepRegisterPaladinNode,
	StepRenderBankEnv,
	StepStartBankInfra,
	StepProvisionBankKeycloak,
	StepStartBackend,
	StepStartBankFrontend,
	StepCreatePenteJoin,
	StepDeployFXAJoin,
	StepGenCSR,
	StepStartLauncher,
}

// PlannedStepOrder returns the step sequence a run will actually execute for the
// given mode and proxy setting. It is the single source both the dry-run plan and
// the apply report are built from: previously each assembled its own order, and
// only the dry-run path remembered to account for the reverse-proxy step — so a
// proxy-enabled run executed a step its own report did not list.
//
// The returned slice is always a fresh copy; callers may append to it without
// mutating the package-level canonical orders.
func PlannedStepOrder(mode string, proxyEnabled bool) []string {
	base := CanonicalStepOrder
	if mode == "join" {
		base = CanonicalJoinStepOrder
	}
	order := make([]string, len(base), len(base)+1)
	copy(order, base)
	// buildSteps / buildJoinSteps append the reverse-proxy step last, and only
	// when spec.proxy == "enable".
	if proxyEnabled {
		order = append(order, StepStartProxy)
	}
	return order
}

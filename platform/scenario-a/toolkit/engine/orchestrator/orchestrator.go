// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/genesis"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
)

// ErrGenesisCorrupt is returned when SPOKE_DATA_DIR/genesis/genesis.json exists
// but is empty or not valid JSON. mode:found refuses to proceed on a corrupt
// genesis rather than letting genesis-init regenerate it (FIX-2: non-destructive).
var ErrGenesisCorrupt = errors.New("orchestrator: genesis.json is present but corrupt — remove it and re-run, or restore a valid genesis")

// DefaultTimeouts returns the default per-step timeout configuration.
// Exposed here for documentation convenience; canonical source is deps.go.
var defaultTimeoutsDoc = DefaultTimeouts()

// RunFound provisions a new spoke in mode:found (central bank founding a new network).
// It executes the 11-step sequence idempotently: each step is skipped if already complete.
// The first step (start-besu) brings up the spoke bootnode and generates genesis,
// so the network is created and ready to operate from the manifest alone — no
// externally pre-started Besu node is required.
//
// Preconditions (caller's responsibility):
//   - Docker + Compose are available (start-besu and start-paladin drive `docker compose`).
//   - deps.CentralBankComposePath points at the TK-4 central-bank docker-compose.yaml.
//   - deps.KeyProvider, deps.CertSource, deps.RelayRegistrar are non-nil.
//   - deps.PaladinCBURL is non-empty (e.g. "http://localhost:31648").
//
// Returns nil on success (all 11 steps done).
// Returns ErrGenesisCorrupt if SPOKE_DATA_DIR/genesis/genesis.json exists but is corrupt.
// Returns ErrProvisioningLocked if another process holds the file lock.
// Returns a wrapped step error on step failure.
func RunFound(ctx context.Context, m *manifest.Manifest, deps Deps) error {
	return runFoundWithSteps(ctx, m, deps, os.Stdout, nil)
}

// runFoundWithSteps is the internal implementation that accepts an explicit step list
// and log writer, enabling unit testing without subprocesses or Docker.
// When steps is nil, the production step list is constructed from manifest and deps.
func runFoundWithSteps(ctx context.Context, m *manifest.Manifest, deps Deps, w io.Writer, steps []Step) error {
	spokeID := m.Spec.Spoke.ID
	dataDir := m.Spec.Node.DataDir
	deps.Timeouts = deps.Timeouts.resolved()

	// Validate required deps.
	if deps.PaladinCBURL == "" {
		return fmt.Errorf("orchestrator: deps.PaladinCBURL is required")
	}

	// 1. Inspect genesis state (non-destructive). mode:found now owns the Besu
	// lifecycle: the start-besu step (genesis-init) generates genesis.json on the
	// first run, so an ABSENT genesis is expected and allowed. Only a CORRUPT
	// genesis aborts — genesis-init must never overwrite/regenerate it (FIX-2).
	result, err := genesis.GuardGenesis(spokeID, dataDir, false, w)
	if err != nil {
		return fmt.Errorf("orchestrator: genesis check: %w", err)
	}
	if result.Decision == genesis.DecisionAbort {
		return ErrGenesisCorrupt
	}

	// 2. Acquire file lock to prevent concurrent provisioning of the same spoke.
	unlock, err := lockState(dataDir)
	if err != nil {
		return err // ErrProvisioningLocked or I/O error
	}
	defer unlock()

	// 3. Load persisted state.
	state, err := LoadState(dataDir)
	if err != nil {
		return fmt.Errorf("orchestrator: load state: %w", err)
	}
	if state.SpokeID == "" {
		state.SpokeID = spokeID
	}

	// 4. Build production step list if not provided by the caller.
	if steps == nil {
		steps = buildSteps(m, deps, dataDir, state)
	}

	// 5. Execute each step: check → skip or run → persist.
	for _, step := range steps {
		if err := ctx.Err(); err != nil {
			return err
		}

		done, err := step.Check(ctx)
		if err != nil {
			return fmt.Errorf("step %s: check: %w", step.Name(), err)
		}
		if done {
			logSkipped(w, spokeID, step.Name())
			// Persist the step as done so a stale "failed"/"pending" from an earlier
			// run converges to the real state on idempotent re-runs.
			state = markStep(state, step.Name(), "done", time.Now().UTC().Format(time.RFC3339))
			if err := saveState(dataDir, state); err != nil {
				return fmt.Errorf("step %s: save state: %w", step.Name(), err)
			}
			continue
		}

		logStarted(w, spokeID, step.Name())
		runErr := step.Run(ctx)
		if runErr != nil {
			// register-relay is a hard step: registering the spoke with the relay
			// is required for the founded network to be reachable by counterparties,
			// so a failure here aborts the apply rather than being swallowed.
			state = markStep(state, step.Name(), "failed", "")
			_ = saveState(dataDir, state)
			logFailed(w, spokeID, step.Name(), runErr)
			return fmt.Errorf("step %s: %w", step.Name(), runErr)
		}

		state = markStep(state, step.Name(), "done", time.Now().UTC().Format(time.RFC3339))
		if err := saveState(dataDir, state); err != nil {
			return fmt.Errorf("step %s: save state: %w", step.Name(), err)
		}
		logCompleted(w, spokeID, step.Name())
	}

	return nil
}

// buildSteps constructs the ordered list of production Step implementations.
// Each step is passed only the data it needs; no global state.
func buildSteps(m *manifest.Manifest, deps Deps, dataDir string, _ ProvisioningState) []Step {
	spokeID := m.Spec.Spoke.ID
	besuRPCURL := deps.BesuRPCURL
	var besuRPCPort, besuWSPort, besuP2PPort int
	if m.Spec.Node.RPC != nil {
		besuRPCPort = m.Spec.Node.RPC.Port
	}
	if m.Spec.Node.WS != nil {
		besuWSPort = m.Spec.Node.WS.Port
	}
	if m.Spec.Node.P2P != nil {
		besuP2PPort = m.Spec.Node.P2P.Port
	}

	besuImage := deps.BesuImage
	if besuImage == "" {
		besuImage = defaultBesuImage
	}

	steps := []Step{
		newStartBesuFoundStep(spokeID, m.Spec.Spoke.ChainID, dataDir, deps.CentralBankComposePath, besuRPCURL,
			m.Spec.Node.AdvertisedHost, besuImage, besuRPCPort, besuWSPort, besuP2PPort,
			deps.Timeouts.PaladinHealthCheck, deps.Timeouts.PaladinHealthCheckInterval),
		newDeployContractsStep(spokeID, dataDir, besuRPCURL, deps.ScriptsDir, deps.Timeouts.GoTestStep),
		newGenTLSStep(spokeID, dataDir, deps.CertSource, deps.KeyProvider),
		newRenderConfigsStep(spokeID, dataDir, besuRPCPort, besuWSPort, deps.PaladinConfigTemplateDir),
		newRegisterNodesStep(spokeID, dataDir, besuRPCURL, deps.KeyProvider, deps.Timeouts.OnboardRegistry),
		newStartPaladinStep(spokeID, dataDir, deps.ComposeTemplatePath, deps.PaladinCBURL, deps.PaladinImage, deps.Timeouts.PaladinHealthCheck, deps.Timeouts.PaladinHealthCheckInterval),
		newCreateZetoStep(spokeID, dataDir, deps.PaladinCBURL, deps.ScriptsDir, deps.Timeouts.GoTestStep),
		newOnboardRegistryStep(spokeID, dataDir, besuRPCURL, deps.KeyProvider,
			filepath.Join(deps.ContractsOutDir, "IdentityRegistry.sol", "IdentityRegistry.json"),
			deps.Timeouts.OnboardRegistry),
		newRegisterRelayStep(spokeID, dataDir, deps.BesuRPCURL, deps.RelayRegistrar, deps.Timeouts.RelayRegistration),
	}

	// CB operational stack (feature 034 US1): dedicated infra + Keycloak + backend.
	// Template/context paths are derived from the Scenario A root (ContractsOutDir
	// is <root>/contracts/out).
	root := filepath.Dir(filepath.Dir(deps.ContractsOutDir))
	templatesDir := filepath.Join(root, "provisioning", "templates")
	entity := m.Metadata.Name
	prefix := entityContainerPrefix(entity)
	net := entityNetName(entity)
	ports := entityPorts(besuRPCPort)
	dbName := entityDBName(entity)
	kcDBURL := fmt.Sprintf("jdbc:postgresql://%s-postgres:5432/%s", prefix, dbName)
	stackTO := deps.Timeouts.PaladinHealthCheck
	stackInt := deps.Timeouts.PaladinHealthCheckInterval

	steps = append(steps,
		newRenderCBEnvStep(spokeID, entity, m.Spec.Spoke.Currency, besuRPCPort, dataDir),
		newStartInfraStep(StepStartCBInfra, prefix, net, dataDir,
			filepath.Join(templatesDir, "entity-infra", "infra-compose.yaml"),
			dbName, "default", "default", ports.Postgres, ports.Redis, stackTO),
		newProvisionKeycloakStep(StepProvisionKeycloak, keycloakStepParams{
			EntityPrefix: prefix, NetName: net, DataDir: dataDir,
			ComposePath: filepath.Join(templatesDir, "entity-keycloak", "keycloak-compose.yaml"),
			KCDBURL:     kcDBURL, KCUser: "default", KCPassword: "default",
			HostPort: ports.Keycloak, Realms: centralBankRealmPlans(entity), Timeout: stackTO,
		}),
		newStartBackendStackStep(StepStartCBBackend, backendStackParams{
			EntityPrefix: prefix, NetName: net, BackendContext: filepath.Join(root, "backend"),
			PKIDir:      filepath.Join(dataDir, "tls"),
			EnvFile:     cbEnvPath(dataDir, entity),
			ComposePath: filepath.Join(templatesDir, "entity-backend", "backend-compose.yaml"),
			BankCode:    entity,
			PaladinURL:  hostInternalURL(deps.PaladinCBURL), PaladinIdentity: paladinIdentity(cbNodeName(spokeID)),
			APIPort: ports.APIGateway, AuthPort: ports.AuthGRPC, CompliancePort: ports.ComplianceGRPC, PaymentPort: ports.PaymentGRPC,
			HealthTimeout: stackTO, HealthInterval: stackInt,
		}),
		newStartFrontendStackStep(StepStartCBFrontend, frontendStackParams{
			EntityPrefix: prefix, NetName: net,
			Context:     filepath.Join(root, "frontend"),
			ComposePath: filepath.Join(templatesDir, "entity-frontend", "frontend-compose.yaml"),
			Services: []frontendService{
				{Service: "governance", Port: ports.FrontendPrimary},
				{Service: "treasury", Port: ports.FrontendSecondary},
				{Service: "supervisor", Port: ports.FrontendSupervisor},
				{Service: "noc", Port: ports.FrontendNOC},
			},
			APIURL:        frontendAPIURL(ports.APIGateway),
			APIBase:       frontendAPIBase(ports.APIGateway),
			PortalOwner:   entity + "-operator", FiatSymbol: m.Spec.Spoke.Currency, Institution: entity,
			KeycloakURL:   frontendAPIBase(ports.Keycloak), KeycloakRealm: "cbweb3", KeycloakClient: "cbweb3-noc",
			HealthTimeout: stackTO, HealthInterval: stackInt,
		}),
	)
	return steps
}

// frontendAPIBase / frontendAPIURL are the host-published api-gateway URLs the
// browser uses (frontends are SPAs served on the host). APIURL carries the /api/v1/
// suffix the bank/governance/treasury portals expect; APIBase is the bare origin.
func frontendAPIBase(port int) string { return fmt.Sprintf("http://localhost:%d", port) }
func frontendAPIURL(port int) string  { return frontendAPIBase(port) + "/api/v1/" }

// hostInternalURL rewrites a localhost URL to host.docker.internal so a container
// can reach a host-published port (Paladin/Cacti run as separate compose stacks).
func hostInternalURL(url string) string {
	return strings.Replace(url, "localhost", "host.docker.internal", 1)
}

// ErrBundleNotFound is returned by RunJoin when the join bundle is nil.
var ErrBundleNotFound = errors.New("orchestrator: join bundle is required for mode:join")

// RunJoin provisions a commercial bank into an existing spoke in mode:join.
// It executes the 9-step sequence idempotently: each step is skipped if already
// complete (per persisted state). Unlike RunFound it does NOT require a
// pre-existing genesis — the write-genesis step materializes it from the bundle.
//
// Preconditions (caller's responsibility):
//   - b is a validated join bundle (see bundle.ValidateForJoin).
//   - deps.KeyProvider is non-nil and deps.BankCode is non-empty.
//   - deps.ComposeTemplatePath points at the commercial-bank docker-compose.yaml.
func RunJoin(ctx context.Context, m *manifest.Manifest, b *bundle.JoinBundle, deps JoinDeps) error {
	return runJoinWithSteps(ctx, m, b, deps, os.Stdout, nil)
}

func runJoinWithSteps(ctx context.Context, m *manifest.Manifest, b *bundle.JoinBundle, deps JoinDeps, w io.Writer, steps []Step) error {
	if b == nil {
		return ErrBundleNotFound
	}
	if deps.BankCode == "" {
		return fmt.Errorf("orchestrator: deps.BankCode is required for mode:join")
	}
	spokeID := m.Spec.Spoke.ID
	dataDir := m.Spec.Node.DataDir
	deps.Timeouts = deps.Timeouts.resolved()

	unlock, err := lockState(dataDir)
	if err != nil {
		return err
	}
	defer unlock()

	state, err := LoadState(dataDir)
	if err != nil {
		return fmt.Errorf("orchestrator: load state: %w", err)
	}
	if state.SpokeID == "" {
		state.SpokeID = spokeID
	}

	if steps == nil {
		steps = buildJoinSteps(m, b, deps, dataDir, w)
	}

	for _, step := range steps {
		if err := ctx.Err(); err != nil {
			return err
		}

		done, err := step.Check(ctx)
		if err != nil {
			return fmt.Errorf("step %s: check: %w", step.Name(), err)
		}
		if done {
			logSkipped(w, spokeID, step.Name())
			// Persist the step as done so a stale "failed"/"pending" from an earlier
			// run converges to the real state on idempotent re-runs.
			state = markStep(state, step.Name(), "done", time.Now().UTC().Format(time.RFC3339))
			if err := saveState(dataDir, state); err != nil {
				return fmt.Errorf("step %s: save state: %w", step.Name(), err)
			}
			continue
		}

		logStarted(w, spokeID, step.Name())
		runErr := step.Run(ctx)
		if runErr != nil {
			// Governance-gated identity steps are non-fatal: the bank is fully
			// provisioned and operational. participant registration (proof-of-
			// possession) is performed by the CB on KYC approval, and the CB-signed
			// cert is issued after approval (Governance Portal). Mark pending and
			// continue so the join reports success.
			if isDeferredOnboardingStep(step.Name()) {
				state = markStep(state, step.Name(), "pending", "")
				_ = saveState(dataDir, state)
				logDetail(w, spokeID, step.Name(), "deferred to governance onboarding: "+runErr.Error())
				continue
			}
			state = markStep(state, step.Name(), "failed", "")
			_ = saveState(dataDir, state)
			logFailed(w, spokeID, step.Name(), runErr)
			// start-backend is a soft failure: the bank is already joined and
			// registered on-chain; the backend stack can be started separately.
			if step.Name() == StepStartBackend {
				continue
			}
			return fmt.Errorf("step %s: %w", step.Name(), runErr)
		}

		state = markStep(state, step.Name(), "done", time.Now().UTC().Format(time.RFC3339))
		if err := saveState(dataDir, state); err != nil {
			return fmt.Errorf("step %s: save state: %w", step.Name(), err)
		}
		logCompleted(w, spokeID, step.Name())
	}

	return nil
}

// buildJoinSteps constructs the ordered list of production Step implementations
// for mode:join.
func buildJoinSteps(m *manifest.Manifest, b *bundle.JoinBundle, deps JoinDeps, dataDir string, w io.Writer) []Step {
	spokeID := m.Spec.Spoke.ID
	var rpcPort, wsPort, p2pPort int
	if m.Spec.Node.RPC != nil {
		rpcPort = m.Spec.Node.RPC.Port
	}
	if m.Spec.Node.WS != nil {
		wsPort = m.Spec.Node.WS.Port
	}
	if m.Spec.Node.P2P != nil {
		p2pPort = m.Spec.Node.P2P.Port
	}

	// "build" / empty is a directive to use the locally-pinned image, not a pullable
	// repository — resolve it to the default Besu image (same as mode:found).
	besuImage := m.Spec.Image
	if besuImage == "" || besuImage == "build" {
		besuImage = defaultBesuImage
	}

	// Local single-host adaptation (feature 018): the bundle carries the CB's
	// in-Docker advertised host. Rewrite endpoints so Besu peers over the shared
	// network (internal P2P port) while the host-run toolkit reaches the CB via
	// published host ports. Non-local profiles use the bundle endpoints verbatim.
	ep := rawEndpoints(b)
	if m.Spec.Environment == "local" {
		if l, err := localizeBundleEndpoints(b); err != nil {
			logDetail(w, spokeID, "localize-endpoints", err.Error())
		} else {
			ep = l
		}
	}

	steps := []Step{
		newWriteGenesisStep(dataDir, b.Spec.Genesis.Content, b.Spec.Genesis.Hash),
		newStartBesuJoinStep(spokeID, deps.BankCode, dataDir, deps.ComposeTemplatePath, deps.BesuRPCURL,
			ep.bootnodeEnode, m.Spec.Node.AdvertisedHost, besuImage, rpcPort, wsPort, p2pPort),
		newWaitSyncStep(deps.BesuRPCURL, 1, deps.Timeouts.WaitSync, deps.Timeouts.WaitSyncInterval, w),
		// Commercial banks join as full nodes (not QBFT validators): the central bank
		// is the spoke's sole validator. This keeps consensus liveness independent of
		// any bank's availability and lets many banks join without a validator-set
		// majority vote. (vote-qbft is retained for a future validator-join mode.)
		// US2 — dynamic Paladin node bring-up for the joining bank.
		newGenTLSJoinStep(spokeID, deps.BankCode, dataDir),
		newRenderConfigJoinStep(spokeID, deps.BankCode, dataDir, deps.BesuRPCPort, deps.BesuWSPort,
			b.Spec.Contracts.RegistryAddress, b.Spec.Contracts.ZetoFactoryAddress, b.Spec.Contracts.PenteFactoryAddress,
			deps.PaladinConfigTemplateDir),
		newStartPaladinJoinStep(spokeID, deps.BankCode, dataDir, deps.PaladinComposePath, deps.PaladinImage,
			deps.BesuRPCPort, deps.Timeouts.WaitSync, deps.Timeouts.WaitSyncInterval),
		newRegisterPaladinNodeStep(spokeID, deps.BankCode, dataDir, deps.BesuRPCURL,
			b.Spec.Contracts.RegistryAddress, deps.KeyProvider, deps.Timeouts.ProofOfPossession),
	}

	// Commercial bank operational stack (feature 034 US2): dedicated infra +
	// per-entity Keycloak + the 4 backend services, in commercial-bank mode.
	// Template/context paths are derived from the Scenario A root (ContractsOutDir
	// is <root>/contracts/out), mirroring the CB found path.
	root := filepath.Dir(filepath.Dir(deps.ContractsOutDir))
	templatesDir := filepath.Join(root, "provisioning", "templates")
	bank := deps.BankCode
	prefix := entityContainerPrefix(bank)
	net := entityNetName(bank)
	ports := entityPorts(deps.BesuRPCPort)
	dbName := entityDBName(bank)
	kcDBURL := fmt.Sprintf("jdbc:postgresql://%s-postgres:5432/%s", prefix, dbName)
	stackTO := deps.Timeouts.WaitSync
	stackInt := deps.Timeouts.WaitSyncInterval

	steps = append(steps,
		newRenderBankEnvStep(bankEnvParams{
			SpokeID: spokeID, BankCode: bank, Currency: b.Spec.Currency,
			BesuRPCPort: deps.BesuRPCPort, BesuRPCURL: deps.BesuRPCURL, DataDir: dataDir,
			CentralBankAPIURL:          ep.cbAPIBaseForBank,
			ZetoTokenAddress:           b.Spec.Contracts.ZetoTokenAddress,
			ParticipantRegistryAddress: b.Spec.Contracts.ParticipantRegistryAddress,
		}),
		newStartInfraStep(StepStartBankInfra, prefix, net, dataDir,
			filepath.Join(templatesDir, "entity-infra", "infra-compose.yaml"),
			dbName, "default", "default", ports.Postgres, ports.Redis, stackTO),
		newProvisionKeycloakStep(StepProvisionBankKeycloak, keycloakStepParams{
			EntityPrefix: prefix, NetName: net, DataDir: dataDir,
			ComposePath: filepath.Join(templatesDir, "entity-keycloak", "keycloak-compose.yaml"),
			KCDBURL:     kcDBURL, KCUser: "default", KCPassword: "default",
			HostPort: ports.Keycloak, Realms: []KeycloakRealmPlan{commercialBankRealmPlan(bank)}, Timeout: stackTO,
		}),
		newStartBackendStackStep(StepStartBackend, backendStackParams{
			EntityPrefix: prefix, NetName: net, BackendContext: filepath.Join(root, "backend"),
			EnvFile:     cbEnvPath(dataDir, bank),
			ComposePath: filepath.Join(templatesDir, "entity-backend", "backend-compose.yaml"),
			BankCode:    bank,
			PaladinURL:  hostInternalURL(bankPaladinURL(deps.BesuRPCPort)), PaladinIdentity: paladinIdentity(bankNodeName(spokeID, bank)),
			APIPort: ports.APIGateway, AuthPort: ports.AuthGRPC, CompliancePort: ports.ComplianceGRPC, PaymentPort: ports.PaymentGRPC,
			HealthTimeout: stackTO, HealthInterval: stackInt,
		}),
		newStartFrontendStackStep(StepStartBankFrontend, frontendStackParams{
			EntityPrefix: prefix, NetName: net,
			Context:     filepath.Join(root, "frontend"),
			ComposePath: filepath.Join(templatesDir, "entity-frontend", "frontend-compose.yaml"),
			Services:    []frontendService{{Service: "bank", Port: ports.FrontendPrimary}},
			APIURL:      frontendAPIURL(ports.APIGateway), APIBase: frontendAPIBase(ports.APIGateway),
			PortalOwner: bank + "-operator", FiatSymbol: b.Spec.Currency, Institution: bank,
			HealthTimeout: stackTO, HealthInterval: stackInt,
		}),
	)

	// Deferred-tail steps are LAST and off the critical path: the bank is fully
	// provisioned above. All are soft (see RunJoin):
	//   - create-pente/deploy-fxa: bilateral CB↔bank Pente + FXAgreement, blocked by
	//     a Paladin cross-node registry-resolution behaviour (the bank node cannot
	//     resolve the remote CB node in pgroup_createGroup); not consumed by the
	//     backend, tracked for Paladin follow-up;
	//   - proof-of-possession: participant registration, performed by the CB on KYC
	//     approval; the CB-signed cert is issued after approval (Governance Portal).
	steps = append(steps,
		newCreatePenteJoinStep(spokeID, deps.BankCode, dataDir, bankPaladinURL(deps.BesuRPCPort), deps.Timeouts.VoteQBFT),
		newDeployFXAJoinStep(spokeID, deps.BankCode, dataDir, bankPaladinURL(deps.BesuRPCPort),
			filepath.Join(deps.ContractsOutDir, "FXAgreement.sol", "FXAgreement.json"),
			b.Spec.Contracts.ParticipantRegistryAddress, deps.Timeouts.VoteQBFT),
		newProofPossessionStep(deps.BankCode, b.Spec.Contracts.RegistryAddress, deps.BesuRPCURL, deps.KeyProvider, deps.Timeouts.ProofOfPossession),
		newGenCSRStep(deps.BankCode, deps.Institution, dataDir),
		newRequestCertStep(deps.BankCode, deps.Institution, dataDir, ep.cbCertEndpoint, deps.KeyProvider, deps.Timeouts.RequestCert),
		newReceiveCertStep(deps.BankCode, dataDir, ep.cbCertEndpoint, deps.Timeouts.ReceiveCert, deps.Timeouts.ReceiveCertInterval, deps.Timeouts.RequestCert),
	)
	return steps
}

// centralBankAPIURL derives the CB api-gateway base URL a commercial bank uses for
// onboarding/proxy, from the credential-request endpoint embedded in the bundle.
func centralBankAPIURL(cbEndpoint string) string {
	if i := strings.Index(cbEndpoint, "/api/v1/"); i >= 0 {
		return cbEndpoint[:i]
	}
	return cbEndpoint
}

// bankPaladinURL is the host RPC URL of the joining bank's Paladin node, derived
// from its Besu RPC host port (see startPaladinJoinStep port scheme).
func bankPaladinURL(besuRPCPort int) string {
	return fmt.Sprintf("http://localhost:%d", besuRPCPort+bankPaladinRPCPortOffset)
}

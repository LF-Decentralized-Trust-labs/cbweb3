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
			EnvFile:     cbEnvPath(dataDir, entity),
			ComposePath: filepath.Join(templatesDir, "entity-backend", "backend-compose.yaml"),
			BankCode:    entity,
			PaladinURL:  hostInternalURL(deps.PaladinCBURL), PaladinIdentity: paladinIdentity(cbNodeName(spokeID)),
			APIPort: ports.APIGateway, AuthPort: ports.AuthGRPC, CompliancePort: ports.ComplianceGRPC, PaymentPort: ports.PaymentGRPC,
			HealthTimeout: stackTO, HealthInterval: stackInt,
		}),
	)
	return steps
}

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
			continue
		}

		logStarted(w, spokeID, step.Name())
		runErr := step.Run(ctx)
		if runErr != nil {
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

	return []Step{
		newWriteGenesisStep(dataDir, b.Spec.Genesis.Content, b.Spec.Genesis.Hash),
		newStartBesuJoinStep(spokeID, deps.BankCode, dataDir, deps.ComposeTemplatePath, deps.BesuRPCURL,
			b.Spec.Bootnode.Enode, m.Spec.Node.AdvertisedHost, m.Spec.Image, rpcPort, wsPort, p2pPort),
		newWaitSyncStep(deps.BesuRPCURL, 1, deps.Timeouts.WaitSync, deps.Timeouts.WaitSyncInterval, w),
		newVoteQBFTStep(spokeID, deps.BesuRPCURL, b.Spec.Validators, deps.Timeouts.VoteQBFT, deps.Timeouts.VoteQBFTInterval, w),
		newGenCSRStep(deps.BankCode, deps.Institution, dataDir),
		newRequestCertStep(deps.BankCode, dataDir, b.Spec.CBEndpoint, deps.KeyProvider, deps.Timeouts.RequestCert),
		newReceiveCertStep(deps.BankCode, dataDir, b.Spec.CBEndpoint, deps.Timeouts.ReceiveCert, deps.Timeouts.ReceiveCertInterval, deps.Timeouts.RequestCert),
		newProofPossessionStep(deps.BankCode, b.Spec.Contracts.RegistryAddress, deps.BesuRPCURL, deps.KeyProvider, deps.Timeouts.ProofOfPossession),
		// US2 — dynamic Paladin node bring-up for the joining bank.
		newGenTLSJoinStep(spokeID, deps.BankCode, dataDir),
		newRenderConfigJoinStep(spokeID, deps.BankCode, dataDir, deps.BesuRPCPort, deps.BesuWSPort,
			b.Spec.Contracts.RegistryAddress, b.Spec.Contracts.ZetoFactoryAddress, b.Spec.Contracts.PenteFactoryAddress,
			deps.PaladinConfigTemplateDir),
		newStartPaladinJoinStep(spokeID, deps.BankCode, dataDir, deps.PaladinComposePath, deps.PaladinImage,
			deps.BesuRPCPort, deps.Timeouts.WaitSync, deps.Timeouts.WaitSyncInterval),
		newRegisterPaladinNodeStep(spokeID, deps.BankCode, dataDir, deps.BesuRPCURL,
			b.Spec.Contracts.RegistryAddress, deps.KeyProvider, deps.Timeouts.ProofOfPossession),
		// US3 — bilateral Pente context + FXAgreement, against the bank's Paladin.
		newCreatePenteJoinStep(spokeID, deps.BankCode, dataDir, bankPaladinURL(deps.BesuRPCPort), deps.Timeouts.VoteQBFT),
		newDeployFXAJoinStep(spokeID, deps.BankCode, dataDir, bankPaladinURL(deps.BesuRPCPort),
			filepath.Join(deps.ContractsOutDir, "FXAgreement.sol", "FXAgreement.json"),
			b.Spec.Contracts.ParticipantRegistryAddress, deps.Timeouts.VoteQBFT),
		newStartBackendStep(spokeID, deps.BankCode, dataDir, deps.BackendComposePath, w),
	}
}

// bankPaladinURL is the host RPC URL of the joining bank's Paladin node, derived
// from its Besu RPC host port (see startPaladinJoinStep port scheme).
func bankPaladinURL(besuRPCPort int) string {
	return fmt.Sprintf("http://localhost:%d", besuRPCPort+bankPaladinRPCPortOffset)
}

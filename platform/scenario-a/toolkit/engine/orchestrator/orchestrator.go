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
		newStartBesuFoundStep(spokeID, m.Spec.Spoke.ChainID, deps.CentralBankComposePath, besuRPCURL,
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
		// Besu-layer settlement contracts (Scenario A): fCeBM + HTLC. deploy-htlc
		// reads PARTICIPANT_REGISTRY_ADDRESS produced by onboard-registry above.
		newDeployFiatTokenStep(spokeID, dataDir, besuRPCURL, m.Spec.Spoke.Currency, deps.KeyProvider,
			filepath.Join(deps.ContractsOutDir, "FiatCentralBankMoney.sol", "FiatCentralBankMoney.json"),
			deps.Timeouts.OnboardRegistry),
		newDeployHTLCStep(dataDir, besuRPCURL, deps.KeyProvider,
			filepath.Join(deps.ContractsOutDir, "HashTimeLockedContract.sol", "HashTimeLockedContract.json"),
			deps.Timeouts.OnboardRegistry),
	}

	// Local operator key wires the backend Besu-signing path (HTLC/fCeBM). Empty
	// off local — leaves the path disabled, matching prior behaviour.
	operatorKeyHex := resolveOperatorKeyHex(deps.KeyProvider)

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
		newRenderCBEnvStep(spokeID, entity, m.Spec.Spoke.Currency, besuRPCPort, m.Spec.Spoke.ChainID, dataDir, operatorKeyHex, frontendAdvertisedHost(m)),
		newStartInfraStep(StepStartCBInfra, prefix, net, dataDir,
			filepath.Join(templatesDir, "entity-infra", "infra-compose.yaml"),
			dbName, "default", "default", ports.Postgres, ports.Redis, stackTO),
		newProvisionKeycloakStep(StepProvisionKeycloak, keycloakStepParams{
			EntityPrefix: prefix, NetName: net, DataDir: dataDir,
			ComposePath: filepath.Join(templatesDir, "entity-keycloak", "keycloak-compose.yaml"),
			KCDBURL:     kcDBURL, KCUser: "default", KCPassword: "default",
			HostPort: ports.Keycloak, Realms: centralBankRealmPlans(entity, m.Spec.AdminUsers), Timeout: stackTO,
		}),
		newStartBackendStackStep(StepStartCBBackend, backendStackParams{
			EntityPrefix: prefix, NetName: net, BackendContext: filepath.Join(root, "backend"),
			PKIDir:      filepath.Join(dataDir, "tls"),
			EnvFile:     cbEnvPath(dataDir, entity),
			ComposePath: filepath.Join(templatesDir, "entity-backend", "backend-compose.yaml"),
			BankCode:    entity,
			PaladinURL:  hostInternalURL(deps.PaladinCBURL), PaladinIdentity: paladinIdentity(cbNodeName(spokeID)),
			// Route payment-orchestrator → relay via spec.relay.endpoint so a remote
			// relay (multi-host deployment) is reachable; falls back to host.docker.internal
			// when the endpoint is empty or uses localhost (co-located relay).
			CactiURL: cactiContainerURL(manifestRelayEndpoint(m)),
			// Un-gate the payment-orchestrator Besu path (HTLC/fCeBM) in local, where an
			// operator key is available; the backend reaches Besu via host.docker.internal.
			PaymentOrchBesuRPCURL: besuPathURL(operatorKeyHex, deps.BesuRPCURL),
			APIPort:               ports.APIGateway, AuthPort: ports.AuthGRPC, CompliancePort: ports.ComplianceGRPC, PaymentPort: ports.PaymentGRPC,
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
			APIURL:      frontendAPIURL(frontendAdvertisedHost(m), ports.APIGateway),
			APIBase:     frontendAPIBase(frontendAdvertisedHost(m), ports.APIGateway),
			PortalOwner: entity + "-operator", FiatSymbol: m.Spec.Spoke.Currency, Institution: m.DisplayNameOr(entity),
			KeycloakURL: frontendAPIBase(frontendAdvertisedHost(m), ports.Keycloak), KeycloakRealm: "cbweb3", KeycloakClient: "cbweb3-noc",
			// Per-entity image tag: VITE_* are baked at build time, so a shared tag
			// would let one entity's bundle (with its api-gateway URL) be reused by
			// another, sending the browser to the wrong gateway and failing CORS.
			ImageTag:      entity,
			HealthTimeout: stackTO, HealthInterval: stackInt,
		}),
	)

	// Register the spoke with the relay LAST, once the CB coordinator endpoints exist. The relay
	// reaches all of these on the host-published ports. It polls InternalApiURL for the CB's
	// FX-agreement aggregate and drives the dest leg via GRPCEndpoint. The host defaults to
	// host.docker.internal (relay co-located on this Docker host); spec.relay.advertisedHost
	// overrides it with a routable IP/hostname when the relay runs elsewhere.
	relayHost := relayAdvertisedHost(m)
	steps = append(steps,
		newRegisterRelayStep(spokeID, dataDir,
			rewriteHost(deps.BesuRPCURL, relayHost),
			fmt.Sprintf("ws://%s:%d", relayHost, besuWSPort),
			fmt.Sprintf("%s:%d", relayHost, ports.PaymentGRPC),
			fmt.Sprintf("http://%s:%d", relayHost, ports.APIGateway),
			deps.RelayRegistrar, deps.Timeouts.RelayRegistration),
	)
	return steps
}

// relayAdvertisedHost returns the host the relay should use to reach this spoke's
// host-published endpoints during register-relay. It is spec.relay.advertisedHost
// when set, otherwise host.docker.internal (the relay is co-located on this Docker
// host and reaches published ports via the host loopback).
func relayAdvertisedHost(m *manifest.Manifest) string {
	if m.Spec.Relay != nil && m.Spec.Relay.AdvertisedHost != "" {
		return m.Spec.Relay.AdvertisedHost
	}
	return dockerHostAlias
}

// frontendAdvertisedHost returns the hostname baked into VITE_API_URL and
// VITE_KEYCLOAK_URL at build time. It is spec.frontendHost when set, otherwise
// "localhost" (which only works when the browser runs on the Docker host itself).
// Set spec.frontendHost to a routable IP or DNS name for remote access.
func frontendAdvertisedHost(m *manifest.Manifest) string {
	if m.Spec.FrontendHost != "" {
		return m.Spec.FrontendHost
	}
	return "localhost"
}

// frontendAPIBase / frontendAPIURL are the host-published api-gateway URLs baked
// into the frontend SPA bundle at build time. APIURL carries the /api/v1/ suffix
// the bank/governance/treasury portals expect; APIBase is the bare origin.
func frontendAPIBase(host string, port int) string { return fmt.Sprintf("http://%s:%d", host, port) }
func frontendAPIURL(host string, port int) string  { return frontendAPIBase(host, port) + "/api/v1/" }

// cbCORSOrigins is the comma-separated browser origin list a central bank's
// api-gateway must allow. A CB serves FOUR portals (governance, treasury,
// supervisor, noc — see startCBFrontend in this file); every one calls the
// gateway from the host, so all four origins must be whitelisted or the omitted
// portals fail CORS at login. When frontendHost differs from "localhost" (e.g.
// a cloud VM with a public IP), both the localhost and the routable-host origins
// are included so the same stack works from both the Docker host and remote browsers.
// (A commercial bank serves only FrontendPrimary, so its list is built in
// renderBankEnvStep.)
func cbCORSOrigins(p EntityPorts, frontendHost string) string {
	local := fmt.Sprintf("http://localhost:%d,http://localhost:%d,http://localhost:%d,http://localhost:%d",
		p.FrontendPrimary, p.FrontendSecondary, p.FrontendSupervisor, p.FrontendNOC)
	if frontendHost == "" || frontendHost == "localhost" {
		return local
	}
	remote := fmt.Sprintf("http://%s:%d,http://%s:%d,http://%s:%d,http://%s:%d",
		frontendHost, p.FrontendPrimary,
		frontendHost, p.FrontendSecondary,
		frontendHost, p.FrontendSupervisor,
		frontendHost, p.FrontendNOC)
	return local + "," + remote
}

// bankCORSOrigins is the comma-separated browser origin list a commercial bank's
// api-gateway must allow. A bank serves one portal (bank/FrontendPrimary) and a
// secondary (FrontendSecondary). When frontendHost is set, both the localhost and
// the routable-host origins are included.
func bankCORSOrigins(p EntityPorts, frontendHost string) string {
	local := fmt.Sprintf("http://localhost:%d,http://localhost:%d", p.FrontendPrimary, p.FrontendSecondary)
	if frontendHost == "" || frontendHost == "localhost" {
		return local
	}
	remote := fmt.Sprintf("http://%s:%d,http://%s:%d", frontendHost, p.FrontendPrimary, frontendHost, p.FrontendSecondary)
	return local + "," + remote
}

// dockerHostAlias is the Docker special DNS name that resolves to the host from
// inside a container, used to reach host-published ports when the relay/Paladin
// run as separate compose stacks on the same Docker host.
const dockerHostAlias = "host.docker.internal"

// hostInternalURL rewrites a localhost URL to host.docker.internal so a container
// can reach a host-published port (Paladin/Cacti run as separate compose stacks).
func hostInternalURL(url string) string {
	return rewriteHost(url, dockerHostAlias)
}

// rewriteHost rewrites the localhost in a URL to the given host, so a caller on a
// different host (e.g. an off-host relay) can reach the spoke's host-published port.
func rewriteHost(url, host string) string {
	return strings.Replace(url, "localhost", host, 1)
}

// cactiContainerURL returns the relay URL that payment-orchestrator containers
// should use to reach the Cacti relay (CACTI_API_URL).
//
// Rules:
//   - Empty endpoint → co-located default (http://host.docker.internal:4000)
//   - "localhost" in endpoint → rewritten to host.docker.internal so the
//     container can reach a co-located relay via the Docker host gateway
//   - External IP / hostname → used as-is (multi-host relay deployment)
func cactiContainerURL(endpoint string) string {
	if endpoint == "" {
		return "http://" + dockerHostAlias + ":4000"
	}
	return hostInternalURL(endpoint) // localhost → host.docker.internal; external URL unchanged
}

// manifestRelayEndpoint returns spec.relay.endpoint from a manifest, or "" when
// the relay section is absent.
func manifestRelayEndpoint(m *manifest.Manifest) string {
	if m.Spec.Relay != nil {
		return m.Spec.Relay.Endpoint
	}
	return ""
}

// bundleRelayEndpoint returns the relay endpoint a commercial bank should use,
// preferring an explicit override in the bank manifest over the bundle value.
func bundleRelayEndpoint(m *manifest.Manifest, b *bundle.JoinBundle) string {
	if m.Spec.Relay != nil && m.Spec.Relay.Endpoint != "" {
		return m.Spec.Relay.Endpoint
	}
	if b.Spec.Relay != nil {
		return b.Spec.Relay.Endpoint
	}
	return ""
}

// besuPathURL returns the container-reachable Besu RPC URL to enable the backend's
// Besu-signing path (HTLC/fCeBM), or "" when no operator key is available (prod),
// which keeps the path disabled. Gating on operatorKeyHex ensures BESU_RPC_URL is
// never set without the operator key the backend requires alongside it.
func besuPathURL(operatorKeyHex, besuRPCURL string) string {
	if operatorKeyHex == "" {
		return ""
	}
	return hostInternalURL(besuRPCURL)
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
		newWriteGenesisStep(spokeID, deps.BankCode, b.Spec.Genesis.Content, b.Spec.Genesis.Hash),
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

	// Per-bank operator key wires the bank backend Besu-signing path (HTLC/fCeBM).
	// DISTINCT per bank (not the shared CB deployer) so each bank has its own on-chain
	// HTLC signer / verified participant. Empty off local — path disabled, as before.
	operatorKeyHex := resolveBankOperatorKeyHex(deps.KeyProvider, spokeID, bank)

	steps = append(steps,
		newRenderBankEnvStep(bankEnvParams{
			SpokeID: spokeID, BankCode: bank, Currency: b.Spec.Currency,
			BesuRPCPort: deps.BesuRPCPort, ChainID: b.Spec.ChainID, BesuRPCURL: deps.BesuRPCURL, DataDir: dataDir,
			CentralBankAPIURL:          ep.cbAPIBaseForBank,
			ZetoTokenAddress:           b.Spec.Contracts.ZetoTokenAddress,
			ParticipantRegistryAddress: b.Spec.Contracts.ParticipantRegistryAddress,
			// fCeBM + HTLC come from the bundle (deployed at found); operator key
			// (local) enables the bank's Besu-signing path for fiat-balance.
			FiatTokenAddress: b.Spec.Contracts.FiatTokenAddress,
			HTLCAddress:      b.Spec.Contracts.HTLCAddress,
			BesuOperatorKey:  operatorKeyHex,
			FrontendHost:     frontendAdvertisedHost(m),
		}),
		newStartInfraStep(StepStartBankInfra, prefix, net, dataDir,
			filepath.Join(templatesDir, "entity-infra", "infra-compose.yaml"),
			dbName, "default", "default", ports.Postgres, ports.Redis, stackTO),
		newProvisionKeycloakStep(StepProvisionBankKeycloak, keycloakStepParams{
			EntityPrefix: prefix, NetName: net, DataDir: dataDir,
			ComposePath: filepath.Join(templatesDir, "entity-keycloak", "keycloak-compose.yaml"),
			KCDBURL:     kcDBURL, KCUser: "default", KCPassword: "default",
			HostPort: ports.Keycloak, Realms: []KeycloakRealmPlan{commercialBankRealmPlan(bank, m.Spec.AdminUsers)}, Timeout: stackTO,
		}),
		newStartBackendStackStep(StepStartBackend, backendStackParams{
			EntityPrefix: prefix, NetName: net, BackendContext: filepath.Join(root, "backend"),
			// Mount the bank's <dataDir>/pki: gen-csr writes <bank>.csr here, which the
			// onboarding smart proxy reads on initiate, and where it persists the issued
			// <bank>-participant.crt on complete (instead of the shared repo pki).
			PKIDir:      filepath.Join(dataDir, "pki"),
			EnvFile:     cbEnvPath(dataDir, bank),
			ComposePath: filepath.Join(templatesDir, "entity-backend", "backend-compose.yaml"),
			BankCode:    bank,
			PaladinURL:  hostInternalURL(bankPaladinURL(deps.BesuRPCPort)), PaladinIdentity: paladinIdentity(bankNodeName(spokeID, bank)),
			// Relay URL for the bank's payment-orchestrator: prefer the bank manifest's
			// relay.endpoint if set; fall back to the bundle value (from the founding CB).
			CactiURL: cactiContainerURL(bundleRelayEndpoint(m, b)),
			// Un-gate the bank payment-orchestrator Besu path (HTLC/fCeBM) in local; it
			// reaches its OWN Besu node (on the spoke chain) via host.docker.internal.
			PaymentOrchBesuRPCURL: besuPathURL(operatorKeyHex, deps.BesuRPCURL),
			APIPort:               ports.APIGateway, AuthPort: ports.AuthGRPC, CompliancePort: ports.ComplianceGRPC, PaymentPort: ports.PaymentGRPC,
			HealthTimeout: stackTO, HealthInterval: stackInt,
		}),
		newStartFrontendStackStep(StepStartBankFrontend, frontendStackParams{
			EntityPrefix: prefix, NetName: net,
			Context:     filepath.Join(root, "frontend"),
			ComposePath: filepath.Join(templatesDir, "entity-frontend", "frontend-compose.yaml"),
			Services:    []frontendService{{Service: "bank", Port: ports.FrontendPrimary}},
			APIURL:      frontendAPIURL(frontendAdvertisedHost(m), ports.APIGateway), APIBase: frontendAPIBase(frontendAdvertisedHost(m), ports.APIGateway),
			PortalOwner: bank + "-operator", FiatSymbol: b.Spec.Currency, Institution: m.DisplayNameOr(bank),
			// Per-entity image tag: VITE_API_URL is baked at build time, so a shared
			// tag would let one bank's bundle be reused by another, pointing the
			// browser at the wrong bank's api-gateway and failing CORS.
			ImageTag:      bank,
			HealthTimeout: stackTO, HealthInterval: stackInt,
		}),
	)

	// Deferred-tail steps are LAST and off the critical path: the bank is fully
	// provisioned above. All are soft (see RunJoin):
	//   - create-pente/deploy-fxa: bilateral CB↔bank Pente + FXAgreement. The cross-node
	//     Paladin transport now works (gen-tls cert CN fixed to the node name — see
	//     E2E-STATUS.md), and create-pente-context gates on Paladin peer-readiness
	//     before creating the group; not consumed by the backend;
	//   - on-chain participant registration is NOT attempted from the join: it is
	//     onlyRole(GOVERNANCE_ROLE) and must register the bank's runtime KMS wallet,
	//     so the CB compliance service performs it (signed by the CB governance key,
	//     CB_PRIVATE_KEY) when it approves the bank's KYC in the Governance Portal;
	//     the CB-signed cert is issued by the portal on that same approval.
	//
	// gen-csr stays: it produces <dataDir>/pki/<bank>.csr, which the bank's
	// api-gateway reads at portal onboarding (initiate) and the CB signs at complete.
	// The toolkit does NOT submit the CSR itself (no request-cert/receive-cert): that
	// would create a second participant record keyed on the toolkit keyProvider wallet
	// instead of the bank's runtime KMS wallet, colliding with the portal's record.
	steps = append(steps,
		// Pente FX-context steps use PenteFXSetup (20m), not VoteQBFT (5m): they run several
		// sequential cross-node-endorsed private txs, and one peer-transport reconnect during the
		// node's initial mesh can stall an endorsement for minutes (observed: a registerParticipant
		// that confirmed 7s after a 5m deadline). See JoinTimeouts.PenteFXSetup.
		newCreatePenteJoinStep(spokeID, deps.BankCode, dataDir, bankPaladinURL(deps.BesuRPCPort), deps.Timeouts.PenteFXSetup, deps.Timeouts.VoteQBFTInterval, w),
		newDeployFXAJoinStep(spokeID, deps.BankCode, dataDir, bankPaladinURL(deps.BesuRPCPort),
			filepath.Join(deps.ContractsOutDir, "FXAgreement.sol", "FXAgreement.json"),
			b.Spec.Contracts.ParticipantRegistryAddress, deps.Timeouts.PenteFXSetup, w),
		newGenCSRStep(deps.BankCode, deps.Institution, dataDir),
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

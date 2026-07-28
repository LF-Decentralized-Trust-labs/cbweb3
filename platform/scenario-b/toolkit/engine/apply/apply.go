// Package apply dispatches an `apply` by manifest mode, runs the orchestrator
// (dry-run or real), and returns a structured Report. It supports the found-hub
// and found-spoke modes; join is TK-B8.
package apply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/manifest"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/orchestrator"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/relayregistrar"
)

// Options configures an apply run.
type Options struct {
	ManifestPath string
	RepoRoot     string // repo root for contracts/templates (default ".")
	DataDir      string // state + lock (default: manifest node.dataDir, else ".")
	OutDir       string // bundle destination (default: DataDir)
	HubRPC       string // hub RPC URL (readiness gate; may be empty in dry-run)
	HubWS        string
	SpokeRPC     string // spoke RPC URL (found-spoke)
	SpokeWS      string // spoke WS URL (found-spoke → relay registration + spoke bundle)
	GatewayURL   string // spoke gateway URL (found-spoke → relay registration)
	CBAddress    string // central bank address (found-spoke → register-cb)
	Relay        string // RelayRegistrar URI (default "local"; e.g. relay://host:4000)
	Format       string // json | yaml
	DryRun       bool
}

// Apply loads+validates the manifest and dispatches by mode.
func Apply(ctx context.Context, o Options) (orchestrator.Report, error) {
	pd, err := manifest.Load(o.ManifestPath)
	if err != nil {
		return orchestrator.Report{}, err
	}
	if res := manifest.Validate(pd); !res.Valid() {
		f := res.Errors[0]
		return orchestrator.Report{}, fmt.Errorf("invalid manifest: %s: %s", f.Field, f.Message)
	}
	switch pd.Spec.Mode {
	case "found-hub":
		return applyFoundHub(ctx, o, pd)
	case "found-spoke":
		return applyFoundSpoke(ctx, o, pd)
	case "join":
		return applyJoin(ctx, o, pd)
	case "observe":
		return applyObserve(ctx, o, pd)
	default:
		return orchestrator.Report{}, fmt.Errorf("unknown mode %q", pd.Spec.Mode)
	}
}

// applyObserve stands up an observe-mode NOC deployment: it consumes the NOC
// bundle (the monitoring topology emitted by a CB's found-spoke / the hub's
// found-hub), brings up the NOC control plane (db + backend + portal), then
// registers the spoke and provisions the founding agent's key. It stands up no
// Besu node, so spec.node is absent — never dereference it here.
func applyObserve(ctx context.Context, o Options, pd *manifest.ParticipantDeployment) (orchestrator.Report, error) {
	if pd.Spec.NOCBundleRef == "" {
		return orchestrator.Report{}, fmt.Errorf("observe: spec.nocBundleRef is required")
	}
	dataDir := absOr(firstNonEmpty(o.DataDir, "."))
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return orchestrator.Report{}, err
	}
	root := absOr(firstNonEmpty(o.RepoRoot, "."))

	// FR-001: consume + validate the NOC bundle BEFORE any effect.
	bundlePath := resolveBundle(pd.Spec.NOCBundleRef, o.ManifestPath)
	nb, err := bundle.LoadNOC(bundlePath)
	if err != nil {
		return orchestrator.Report{}, fmt.Errorf("observe: invalid noc bundle %q: %w", bundlePath, err)
	}

	lock, err := orchestrator.AcquireLock(dataDir)
	if err != nil {
		return orchestrator.Report{}, err
	}
	defer func() { _ = lock.Release() }()

	state, err := orchestrator.LoadState(dataDir)
	if err != nil {
		return orchestrator.Report{}, err
	}

	prefix := sanitizePrefix(pd.Metadata.Name) // e.g. "noc-brazil"
	cfg := orchestrator.ObserveConfig{
		ScenarioBDir:    filepath.Join(root, "scenario-b"),
		TemplatesDir:    filepath.Join(root, "scenario-b", "provisioning", "templates"),
		Bundle:          nb,
		ContainerPrefix: "sc-b-cbweb3-" + prefix,
		NetPrefix:       prefix,
		VolumePrefix:    prefix,
		FrontendHost:    pd.Spec.FrontendHost,
		KeycloakURL:     manifestNOCKeycloakURL(pd), // portal VITE_KEYCLOAK_URL (CB/hub realm)
		LauncherURL:     manifestNOCLauncherURL(pd), // portal VITE_LAUNCHER_URL (back-to-launcher)
		// BackendPort/PortalPort fall back to the local convention in WithDefaults;
		// spec.noc may carry explicit ports in a later phase.
	}
	cfg.WithDefaults()
	if o.DryRun {
		cfg.Runner = &exec.DryRunner{}
	} else {
		cfg.Runner = exec.NewReal(root, cfg.ComposeEnv())
	}

	steps := orchestrator.ObserveSteps(cfg)
	return orchestrator.New("observe", steps, state, o.DryRun).Run(ctx)
}

func applyFoundSpoke(ctx context.Context, o Options, pd *manifest.ParticipantDeployment) (orchestrator.Report, error) {
	if pd.Spec.Spoke == nil {
		return orchestrator.Report{}, fmt.Errorf("found-spoke: spec.spoke is required")
	}
	dataDir := absOr(firstNonEmpty(o.DataDir, pd.Spec.Node.DataDir, "."))
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return orchestrator.Report{}, err
	}
	outDir := absOr(firstNonEmpty(o.OutDir, dataDir))
	root := absOr(firstNonEmpty(o.RepoRoot, "."))

	// FR-001: consume + validate the hub bundle BEFORE any effect.
	hubBundlePath := resolveHubBundle(pd.Spec.HubBundleRef, o.ManifestPath)
	hub, err := bundle.LoadHub(hubBundlePath)
	if err != nil {
		return orchestrator.Report{}, fmt.Errorf("found-spoke: invalid hub bundle %q: %w", hubBundlePath, err)
	}
	hubRPC := firstNonEmpty(o.HubRPC, hub.HubRPC)

	// Relay is external (scenario-a strategy): its address comes from the manifest
	// spec.relay.endpoint (an --relay flag overrides). Absent → local in-memory.
	relayURI := firstNonEmpty(o.Relay, manifestRelayEndpoint(pd), "local")
	reg, err := relayregistrar.New(relayURI)
	if err != nil {
		return orchestrator.Report{}, err
	}

	lock, err := orchestrator.AcquireLock(dataDir)
	if err != nil {
		return orchestrator.Report{}, err
	}
	defer func() { _ = lock.Release() }()

	state, err := orchestrator.LoadState(dataDir)
	if err != nil {
		return orchestrator.Report{}, err
	}

	rpcPort, wsPort, p2pPort := nodePorts(pd.Spec.Node)
	prefix := sanitizePrefix(pd.Metadata.Name) // e.g. "central-bank-brazil"
	cfg := orchestrator.SpokeConfig{
		ContractsDir:        filepath.Join(root, "scenario-b", "contracts"),
		TemplatesDir:        filepath.Join(root, "scenario-b", "provisioning", "templates"),
		OutDir:              outDir,
		SpokeID:             pd.Spec.Spoke.ID,
		SpokeChainID:        uint64(pd.Spec.Spoke.ChainID),
		SpokeRPC:            firstNonEmpty(o.SpokeRPC, localRPC(rpcPort)),
		SpokeWS:             firstNonEmpty(o.SpokeWS, localWS(wsPort)), // FR-008: first-class spoke WS (relay registration + bundle)
		CBAddress:           o.CBAddress,
		HubBundlePath:       hubBundlePath,
		HubRPC:              hubRPC,
		SpokeEnvFile:        filepath.Join(dataDir, ".env.spoke"),
		KeycloakEnv:         []string{filepath.Join(dataDir, ".env.spoke")},
		GatewayURL:          o.GatewayURL,
		Registrar:           reg,
		VolumePrefix:        prefix,
		ContainerPrefix:     "sc-b-cbweb3-" + prefix,
		NetPrefix:           prefix,
		Entity:              firstNonEmpty(pd.Spec.Topology.Role, "central-bank"),
		RPCPort:             rpcPort,
		WSPort:              wsPort,
		P2PPort:             p2pPort,
		AdvertisedHost:      pd.Spec.Node.AdvertisedHost,
		RelayAdvertisedHost: manifestRelayAdvHost(pd),
		RelayEndpoint:       manifestRelayEndpoint(pd), // relay's own REST endpoint → CACTI_API_URL
		FrontendHost:        pd.Spec.FrontendHost,
		LauncherEnabled:     pd.Spec.Launcher == "enable",
		LauncherPort:        pd.Spec.LauncherPort,
		Currency:            pd.Spec.Spoke.Currency,
		AdminUsers:          toOrchestratorAdminUsers(pd.Spec.AdminUsers),
		NOCBackendURL:       manifestNOCBackendURL(pd), // where this CB's noc-agent pushes
		ProxyEnabled:        pd.Spec.Proxy == "enable",
	}
	cfg.WithDefaults()
	if o.DryRun {
		cfg.Runner = &exec.DryRunner{}
	} else {
		// Plumbing (image names, container/network/volume prefixes, host port
		// mappings, static local infra creds) rides on the runner's process env so
		// `docker compose` resolves every ${...} without persisting it to disk;
		// only runtime-discovered values (contract addresses, Keycloak client
		// secret) land in .env.spoke (scenario-a parity).
		cfg.Runner = exec.NewReal(root, cfg.ComposeEnv())
	}

	// The sovereign FX corridor (spec.pair) is opened at runtime via each CB's
	// governance portal (propose/confirm pair + cooperative liquidity), not at
	// provisioning time — the toolkit holds no sovereign signing keys.

	// FR-002/SC-002: wire register-cb's idempotency Check to a live hub probe so
	// a re-apply skips re-registration. Not in dry-run (Check runs before the
	// dry-run branch, so a live probe would be an effect) and only when the hub
	// RPC + CB address are known.
	if !o.DryRun && hubRPC != "" && o.CBAddress != "" {
		identityRegistry := hub.Contracts["identityRegistry"]
		// The probe runs in the HOST toolkit process, so localize the bundle's
		// host.docker.internal hub RPC (a container sentinel) to localhost; a routable
		// multi-VM hub RPC is left unchanged.
		hubProbeRPC := orchestrator.HostReachable(hubRPC)
		cfg.CBRegistered = func(ctx context.Context) (bool, error) {
			return orchestrator.HubCBRegistered(ctx, hubProbeRPC, identityRegistry, o.CBAddress)
		}
	}

	steps := orchestrator.FoundSpokeSteps(cfg)
	steps = append(steps, launcherStep(pd, cfg.Runner, rpcPort))
	if cfg.ProxyEnabled {
		steps = append(steps, orchestrator.NewProxyStep(orchestrator.ProxyParams{
			Runner:       cfg.Runner,
			Mode:         pd.Spec.Proxy,
			SiteHost:     pd.Spec.FrontendHost,
			LauncherPort: pd.Spec.LauncherPort,
			Networks:     []string{cfg.NetName()},
			Routes:       cfg.ProxyRoutes(),
		}))
	}
	return orchestrator.New("found-spoke", steps, state, o.DryRun).Run(ctx)
}

// launcherStep builds the per-entity launcher step from the manifest: it enables/
// disables THIS entity's launcher (distributed A/B entry point) with this scenario's
// portal fragment. TopoRole selects the entity's portals; host + rpcPort derive URLs.
func launcherStep(pd *manifest.ParticipantDeployment, runner exec.CommandRunner, rpcPort int) orchestrator.Step {
	return orchestrator.NewLauncherStep(orchestrator.LauncherParams{
		Runner:   runner,
		Mode:     pd.Spec.Launcher,
		TopoRole: pd.Spec.Topology.Role,
		Entity:   firstNonEmpty(pd.Spec.DisplayName, pd.Metadata.Name),
		Host:     pd.Spec.FrontendHost,
		RPCPort:  rpcPort,
		Port:     pd.Spec.LauncherPort,
		Proxy:    pd.Spec.Proxy == "enable",
	})
}

func applyJoin(ctx context.Context, o Options, pd *manifest.ParticipantDeployment) (orchestrator.Report, error) {
	if pd.Spec.Spoke == nil {
		return orchestrator.Report{}, fmt.Errorf("join: spec.spoke is required")
	}
	dataDir := absOr(firstNonEmpty(o.DataDir, pd.Spec.Node.DataDir, "."))
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return orchestrator.Report{}, err
	}
	outDir := absOr(firstNonEmpty(o.OutDir, dataDir))
	root := absOr(firstNonEmpty(o.RepoRoot, "."))

	// FR-001: consume + validate the spoke bundle BEFORE any effect.
	bundlePath := resolveBundle(pd.Spec.JoinBundleRef, o.ManifestPath)
	sb, err := bundle.LoadSpoke(bundlePath)
	if err != nil {
		return orchestrator.Report{}, fmt.Errorf("join: invalid spoke bundle %q: %w", bundlePath, err)
	}

	lock, err := orchestrator.AcquireLock(dataDir)
	if err != nil {
		return orchestrator.Report{}, err
	}
	defer func() { _ = lock.Release() }()

	state, err := orchestrator.LoadState(dataDir)
	if err != nil {
		return orchestrator.Report{}, err
	}

	rpcPort, wsPort, p2pPort := nodePorts(pd.Spec.Node)
	prefix := sanitizePrefix(pd.Metadata.Name) // e.g. "bank-itau"
	cfg := orchestrator.JoinConfig{
		TemplatesDir:    filepath.Join(root, "scenario-b", "provisioning", "templates"),
		OutDir:          outDir,
		BankID:          pd.Spec.BankID,
		Institution:     firstNonEmpty(pd.Spec.DisplayName, pd.Spec.BankID),
		SpokeID:         pd.Spec.Spoke.ID,
		SpokeChainID:    uint64(pd.Spec.Spoke.ChainID),
		BankRPC:         firstNonEmpty(o.SpokeRPC, localRPC(rpcPort)), // RPC of the bank's own node (wait-sync gate)
		SpokeBundlePath: bundlePath,
		DataDir:         dataDir,
		BankEnvFile:     filepath.Join(dataDir, ".env.bank"),
		KeycloakEnv:     []string{filepath.Join(dataDir, ".env.bank")},
		VolumePrefix:    prefix,
		ContainerPrefix: "sc-b-cbweb3-" + prefix,
		NetPrefix:       prefix,
		Entity:          prefix,
		RPCPort:         rpcPort,
		WSPort:          wsPort,
		P2PPort:         p2pPort,
		HubRPC:          firstNonEmpty(o.HubRPC, sb.HubRPC), // routable hub RPC from the spoke bundle
		RelayEndpoint:   manifestRelayEndpoint(pd),          // the relay's own REST endpoint → CACTI_API_URL
		NOCBackendURL:   manifestNOCBackendURL(pd),          // where this bank's noc-agent pushes
		FrontendHost:    pd.Spec.FrontendHost,
		LauncherEnabled: pd.Spec.Launcher == "enable",
		LauncherPort:    pd.Spec.LauncherPort,
		AdminUsers:      toOrchestratorAdminUsers(pd.Spec.AdminUsers),
		ProxyEnabled:    pd.Spec.Proxy == "enable",
	}
	cfg.WithDefaults()
	if o.DryRun {
		cfg.Runner = &exec.DryRunner{}
	} else {
		// Plumbing (image names, container/network/volume prefixes, host port
		// mappings, the CB bootnode enode, static local infra creds) rides on the
		// runner's process env so `docker compose` resolves every ${...} without
		// persisting it to disk; only runtime-discovered values (contract
		// addresses, Keycloak client secret) land in .env.bank (scenario-a parity).
		cfg.Runner = exec.NewReal(root, cfg.ComposeEnv())
	}
	steps := orchestrator.JoinSteps(cfg)
	steps = append(steps, launcherStep(pd, cfg.Runner, rpcPort))
	if cfg.ProxyEnabled {
		steps = append(steps, orchestrator.NewProxyStep(orchestrator.ProxyParams{
			Runner:       cfg.Runner,
			Mode:         pd.Spec.Proxy,
			SiteHost:     pd.Spec.FrontendHost,
			LauncherPort: pd.Spec.LauncherPort,
			Networks:     []string{cfg.NetName()},
			Routes:       cfg.ProxyRoutes(),
		}))
	}
	return orchestrator.New("join", steps, state, o.DryRun).Run(ctx)
}

// resolveBundle resolves a (possibly relative) bundle ref against the manifest's
// directory.
func resolveBundle(ref, manifestPath string) string {
	if ref == "" || filepath.IsAbs(ref) {
		return ref
	}
	return filepath.Join(filepath.Dir(manifestPath), ref)
}

// resolveHubBundle resolves a (possibly relative) hub bundle ref against the
// manifest's directory.
func resolveHubBundle(ref, manifestPath string) string {
	if ref == "" || filepath.IsAbs(ref) {
		return ref
	}
	return filepath.Join(filepath.Dir(manifestPath), ref)
}

func applyFoundHub(ctx context.Context, o Options, pd *manifest.ParticipantDeployment) (orchestrator.Report, error) {
	dataDir := absOr(firstNonEmpty(o.DataDir, pd.Spec.Node.DataDir, "."))
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return orchestrator.Report{}, err
	}
	outDir := absOr(firstNonEmpty(o.OutDir, dataDir))
	root := absOr(firstNonEmpty(o.RepoRoot, "."))

	lock, err := orchestrator.AcquireLock(dataDir)
	if err != nil {
		return orchestrator.Report{}, err
	}
	defer func() { _ = lock.Release() }()

	state, err := orchestrator.LoadState(dataDir)
	if err != nil {
		return orchestrator.Report{}, err
	}

	var runner exec.CommandRunner
	if o.DryRun {
		runner = &exec.DryRunner{}
	} else {
		runner = exec.NewReal(root, nil)
	}

	rpcPort, wsPort, p2pPort := nodePorts(pd.Spec.Node)
	prefix := sanitizePrefix(pd.Metadata.Name) // e.g. "hub-cbweb3"
	var advertisedHost string
	if pd.Spec.Node != nil {
		advertisedHost = pd.Spec.Node.AdvertisedHost
	}
	cfg := orchestrator.HubConfig{
		Runner:          runner,
		ContractsDir:    filepath.Join(root, "scenario-b", "contracts"),
		TemplatesDir:    filepath.Join(root, "scenario-b", "provisioning", "templates"),
		OutDir:          outDir,
		ChainID:         uint64(pd.Spec.Hub.ChainID),
		HubRPC:          firstNonEmpty(o.HubRPC, localRPC(rpcPort)),
		HubWS:           firstNonEmpty(o.HubWS, localWS(wsPort)),
		HubEnvFile:      filepath.Join(dataDir, ".env.hub"),
		KeycloakEnv:     []string{filepath.Join(dataDir, ".env.hub")},
		VolumePrefix:    prefix,
		ContainerPrefix: "sc-b-cbweb3-" + prefix,
		NetPrefix:       prefix,
		RPCPort:         rpcPort,
		WSPort:          wsPort,
		P2PPort:         p2pPort,
		AdvertisedHost:  advertisedHost,
		FrontendHost:    pd.Spec.FrontendHost,
		ProxyEnabled:    pd.Spec.Proxy == "enable",
	}
	// No launcher on the hub: the launcher is the per-entity A/B entry point for
	// commercial banks and central banks (found-spoke / join), not for the network
	// operator's hub (Scenario B only).
	steps := orchestrator.FoundHubSteps(cfg)
	if cfg.ProxyEnabled {
		// The hub has no launcher landing page, so root redirects to its governance portal.
		steps = append(steps, orchestrator.NewProxyStep(orchestrator.ProxyParams{
			Runner:       runner,
			Mode:         pd.Spec.Proxy,
			SiteHost:     pd.Spec.FrontendHost,
			Networks:     []string{cfg.NetName()},
			Routes:       cfg.ProxyRoutes(),
			RootRedirect: "/b/governance/",
		}))
	}
	return orchestrator.New("found-hub", steps, state, o.DryRun).Run(ctx)
}

// nodePorts extracts the host RPC/WS/P2P ports from the manifest node (0 when unset).
func nodePorts(n *manifest.Node) (rpc, ws, p2p int) {
	if n == nil {
		return 0, 0, 0
	}
	if n.RPC != nil {
		rpc = n.RPC.Port
	}
	if n.WS != nil {
		ws = n.WS.Port
	}
	if n.P2P != nil {
		p2p = n.P2P.Port
	}
	return rpc, ws, p2p
}

// sanitizePrefix makes a metadata name safe for volume/network/container names.
func sanitizePrefix(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.NewReplacer(" ", "-", "_", "-", "/", "-").Replace(s)
	if s == "" {
		return "cbweb3"
	}
	return s
}

func localRPC(port int) string {
	if port == 0 {
		return ""
	}
	return fmt.Sprintf("http://localhost:%d", port)
}

func localWS(port int) string {
	if port == 0 {
		return ""
	}
	return fmt.Sprintf("ws://localhost:%d", port)
}

// absOr resolves p to an absolute path (so it works both as a Go file path and
// as an exec arg regardless of the runner's cwd); returns p unchanged on error.
func absOr(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// manifestRelayEndpoint returns spec.relay.endpoint (the external relay address),
// or "" when no relay block is present.
func manifestRelayEndpoint(pd *manifest.ParticipantDeployment) string {
	if pd.Spec.Relay == nil {
		return ""
	}
	return pd.Spec.Relay.Endpoint
}

// manifestRelayAdvHost returns spec.relay.advertisedHost — the host the external
// relay uses to reach this spoke's published endpoints — defaulting to
// host.docker.internal (relay co-located on the same Docker host).
func manifestRelayAdvHost(pd *manifest.ParticipantDeployment) string {
	if pd.Spec.Relay != nil && pd.Spec.Relay.AdvertisedHost != "" {
		return pd.Spec.Relay.AdvertisedHost
	}
	return "host.docker.internal"
}

// manifestNOCBackendURL returns spec.noc.backendURL (where this entity's
// noc-agent pushes), or "" to fall back to the local single-host convention.
func manifestNOCBackendURL(pd *manifest.ParticipantDeployment) string {
	if pd.Spec.NOC == nil {
		return ""
	}
	return pd.Spec.NOC.BackendURL
}

// manifestNOCKeycloakURL returns spec.noc.keycloakURL (the CB/hub Keycloak the
// NOC portal password-grants against), or "" when unset.
func manifestNOCKeycloakURL(pd *manifest.ParticipantDeployment) string {
	if pd.Spec.NOC == nil {
		return ""
	}
	return pd.Spec.NOC.KeycloakURL
}

// manifestNOCLauncherURL returns spec.noc.launcherURL (the NOC portal's
// back-to-launcher target), or "" when unset.
func manifestNOCLauncherURL(pd *manifest.ParticipantDeployment) string {
	if pd.Spec.NOC == nil {
		return ""
	}
	return pd.Spec.NOC.LauncherURL
}

// toOrchestratorAdminUsers converts the manifest's spec.adminUsers into the
// orchestrator's AdminUser type (the toolkit seeds one Keycloak account per entry).
func toOrchestratorAdminUsers(users []manifest.AdminUser) []orchestrator.AdminUser {
	out := make([]orchestrator.AdminUser, 0, len(users))
	for _, u := range users {
		out = append(out, orchestrator.AdminUser{Role: u.Role, Username: u.Username, Password: u.Password})
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

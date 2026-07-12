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
	default:
		return orchestrator.Report{}, fmt.Errorf("unknown mode %q", pd.Spec.Mode)
	}
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
		ContainerPrefix:     "cbweb3-" + prefix,
		NetPrefix:           prefix,
		Entity:              firstNonEmpty(pd.Spec.Topology.Role, "central-bank"),
		RPCPort:             rpcPort,
		WSPort:              wsPort,
		P2PPort:             p2pPort,
		AdvertisedHost:      pd.Spec.Node.AdvertisedHost,
		RelayAdvertisedHost: manifestRelayAdvHost(pd),
		Currency:            pd.Spec.Spoke.Currency,
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
		cfg.CBRegistered = func(ctx context.Context) (bool, error) {
			return orchestrator.HubCBRegistered(ctx, hubRPC, identityRegistry, o.CBAddress)
		}
	}

	steps := orchestrator.FoundSpokeSteps(cfg)
	return orchestrator.New("found-spoke", steps, state, o.DryRun).Run(ctx)
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
	if _, err := bundle.LoadSpoke(bundlePath); err != nil {
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
		ContainerPrefix: "cbweb3-" + prefix,
		NetPrefix:       prefix,
		Entity:          prefix,
		RPCPort:         rpcPort,
		WSPort:          wsPort,
		P2PPort:         p2pPort,
		HubRPC:          o.HubRPC,
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
		ContainerPrefix: "cbweb3-" + prefix,
		NetPrefix:       prefix,
		RPCPort:         rpcPort,
		WSPort:          wsPort,
		P2PPort:         p2pPort,
	}
	steps := orchestrator.FoundHubSteps(cfg)
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

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

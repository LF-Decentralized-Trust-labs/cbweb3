// Package apply dispatches an `apply` by manifest mode, runs the orchestrator
// (dry-run or real), and returns a structured Report. It supports the found-hub
// and found-spoke modes; join is TK-B8.
package apply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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
	// Sovereign-pair tail (TK-B9; local-only inputs + keys). Empty is fine in dry-run.
	RelayerAddr        string // grant CENTRAL_BANK_ROLE to the relayer on the sovereign tokens
	HubAdminKey        string // scaffolding acts (deploy/grants)
	CBHubKey           string // current CB's sovereign act (propose/confirm/commit)
	ProposerCBAddress  string // proposer CB EVM address (setCentralBankOf tokenA); defaults to CBAddress
	ConfirmerCBAddress string // confirmer CB EVM address (setCentralBankOf tokenB)
	PairRate           string // seed-oracle rate (local)
	CommitAmountA      string // CB-A commit amount (local)
	CommitAmountB      string // CB-B commit amount (local)
	Format             string // json | yaml
	DryRun             bool
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
	dataDir := firstNonEmpty(o.DataDir, pd.Spec.Node.DataDir, ".")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return orchestrator.Report{}, err
	}
	outDir := firstNonEmpty(o.OutDir, dataDir)
	root := firstNonEmpty(o.RepoRoot, ".")

	// FR-001: consume + validate the hub bundle BEFORE any effect.
	hubBundlePath := resolveHubBundle(pd.Spec.HubBundleRef, o.ManifestPath)
	hub, err := bundle.LoadHub(hubBundlePath)
	if err != nil {
		return orchestrator.Report{}, fmt.Errorf("found-spoke: invalid hub bundle %q: %w", hubBundlePath, err)
	}
	hubRPC := firstNonEmpty(o.HubRPC, hub.HubRPC)

	reg, err := relayregistrar.New(firstNonEmpty(o.Relay, "local"))
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

	var runner exec.CommandRunner
	if o.DryRun {
		runner = &exec.DryRunner{}
	} else {
		runner = exec.NewReal(root, nil)
	}

	cfg := orchestrator.SpokeConfig{
		Runner:        runner,
		ContractsDir:  filepath.Join(root, "scenario-b", "contracts"),
		TemplatesDir:  filepath.Join(root, "scenario-b", "provisioning", "templates"),
		OutDir:        outDir,
		SpokeID:       pd.Spec.Spoke.ID,
		SpokeChainID:  uint64(pd.Spec.Spoke.ChainID),
		SpokeRPC:      o.SpokeRPC,
		SpokeWS:       o.SpokeWS, // FR-008: first-class spoke WS (relay registration + bundle)
		CBAddress:     o.CBAddress,
		HubBundlePath: hubBundlePath,
		HubRPC:        hubRPC,
		SpokeEnvFile:  filepath.Join(dataDir, ".env.spoke"),
		KeycloakEnv:   []string{filepath.Join(dataDir, ".env.spoke")},
		GatewayURL:    o.GatewayURL,
		Registrar:     reg,
	}

	// TK-B9: sovereign-pair tail (soft) — populated only when spec.pair is present.
	if pd.Spec.Pair != nil {
		cfg.Environment = pd.Spec.Environment
		cfg.HubPairRegistry = hub.Contracts["pairRegistry"]
		cfg.HubManualOracle = hub.Contracts["manualOracle"]
		cfg.HubIdentityRegistry = hub.Contracts["identityRegistry"]
		cfg.RelayerAddr = o.RelayerAddr
		cfg.HubAdminKey = o.HubAdminKey
		cfg.CBHubKey = o.CBHubKey
		cfg.Pair = &orchestrator.PairConfig{
			ProposerCB:         pd.Spec.Pair.ProposerCB,
			ConfirmerCB:        pd.Spec.Pair.ConfirmerCB,
			ProposerCBAddress:  o.ProposerCBAddress,
			ConfirmerCBAddress: o.ConfirmerCBAddress,
			SymbolA:            pd.Spec.Pair.SymbolA,
			SymbolB:            pd.Spec.Pair.SymbolB,
			CurrentCB:          pd.Metadata.Name, // this run's CB (matches proposerCB/confirmerCB)
			Rate:               o.PairRate,
			AmountA:            o.CommitAmountA,
			AmountB:            o.CommitAmountB,
		}
	}

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
	dataDir := firstNonEmpty(o.DataDir, pd.Spec.Node.DataDir, ".")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return orchestrator.Report{}, err
	}
	outDir := firstNonEmpty(o.OutDir, dataDir)
	root := firstNonEmpty(o.RepoRoot, ".")

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

	var runner exec.CommandRunner
	if o.DryRun {
		runner = &exec.DryRunner{}
	} else {
		runner = exec.NewReal(root, nil)
	}

	cfg := orchestrator.JoinConfig{
		Runner:          runner,
		TemplatesDir:    filepath.Join(root, "scenario-b", "provisioning", "templates"),
		OutDir:          outDir,
		BankID:          pd.Spec.BankID,
		Institution:     firstNonEmpty(pd.Spec.DisplayName, pd.Spec.BankID),
		SpokeID:         pd.Spec.Spoke.ID,
		SpokeChainID:    uint64(pd.Spec.Spoke.ChainID),
		BankRPC:         o.SpokeRPC, // RPC of the bank's own node (wait-sync gate)
		SpokeBundlePath: bundlePath,
		DataDir:         dataDir,
		BankEnvFile:     filepath.Join(dataDir, ".env.bank"),
		KeycloakEnv:     []string{filepath.Join(dataDir, ".env.bank")},
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
	dataDir := firstNonEmpty(o.DataDir, pd.Spec.Node.DataDir, ".")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return orchestrator.Report{}, err
	}
	outDir := firstNonEmpty(o.OutDir, dataDir)
	root := firstNonEmpty(o.RepoRoot, ".")

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

	cfg := orchestrator.HubConfig{
		Runner:       runner,
		ContractsDir: filepath.Join(root, "scenario-b", "contracts"),
		TemplatesDir: filepath.Join(root, "scenario-b", "provisioning", "templates"),
		OutDir:       outDir,
		ChainID:      uint64(pd.Spec.Hub.ChainID),
		HubRPC:       o.HubRPC,
		HubWS:        o.HubWS,
		HubEnvFile:   filepath.Join(dataDir, ".env.hub"),
		KeycloakEnv:  []string{filepath.Join(dataDir, ".env.hub")},
	}
	steps := orchestrator.FoundHubSteps(cfg)
	return orchestrator.New("found-hub", steps, state, o.DryRun).Run(ctx)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// Package apply dispatches an `apply` by manifest mode, runs the orchestrator
// (dry-run or real), and returns a structured Report. This phase supports the
// found-hub mode; found-spoke/join are TK-B7/B8.
package apply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/manifest"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/orchestrator"
)

// Options configures an apply run.
type Options struct {
	ManifestPath string
	RepoRoot     string // repo root for contracts/templates (default ".")
	DataDir      string // state + lock (default: manifest node.dataDir, else ".")
	OutDir       string // bundle destination (default: DataDir)
	HubRPC       string // hub RPC URL (readiness gate; may be empty in dry-run)
	HubWS        string
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
	case "found-spoke", "join":
		return orchestrator.Report{}, fmt.Errorf("mode %q not supported yet (TK-B7/B8)", pd.Spec.Mode)
	default:
		return orchestrator.Report{}, fmt.Errorf("unknown mode %q", pd.Spec.Mode)
	}
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

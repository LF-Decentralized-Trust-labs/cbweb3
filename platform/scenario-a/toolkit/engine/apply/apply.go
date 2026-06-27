// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/orchestrator"
)

// runnerFuncs holds injectable engine functions for testing.
type runnerFuncs struct {
	runFound   func(ctx context.Context, m *manifest.Manifest, deps orchestrator.Deps) error
	emitBundle func(ctx context.Context, in bundle.BundleInput) (*bundle.JoinBundle, error)
}

func defaultRunnerFuncs() runnerFuncs {
	return runnerFuncs{
		runFound:   orchestrator.RunFound,
		emitBundle: bundle.EmitBundle,
	}
}

// Run provisions the spoke described by in.Manifest.
// Calls orchestrator.RunFound, then bundle.EmitBundle.
// Returns a fully populated ApplyResult even on error (partial state captured).
func Run(ctx context.Context, in ApplyInput) (ApplyResult, error) {
	return run(ctx, in, defaultRunnerFuncs())
}

func run(ctx context.Context, in ApplyInput, fns runnerFuncs) (ApplyResult, error) {
	m := in.Manifest
	spokeID := m.Spec.Spoke.ID

	result := ApplyResult{
		Spoke:  spokeID,
		Mode:   m.Spec.Mode,
		DryRun: false,
	}

	// Ensure dataDir exists before any filesystem operation.
	if err := os.MkdirAll(m.Spec.Node.DataDir, 0o755); err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("create data dir: %v", err)
		result.Steps = pendingSteps()
		return result, err
	}

	// Wire deps from profile.
	deps, err := ResolveDeps(m, resolveLocalProfileFromInput(in))
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		result.Steps = pendingSteps()
		return result, err
	}

	// Run the 10-step idempotent provisioning engine.
	runErr := fns.runFound(ctx, m, deps)

	// Read final state to build the step report regardless of error.
	state, _ := orchestrator.LoadState(m.Spec.Node.DataDir)
	result.Steps = buildStepResults(orchestrator.CanonicalStepOrder, state)

	if ctx.Err() != nil {
		// Mark the last step that was running when context was cancelled (recorded
		// as "failed" in the state file) as "interrupted" for per-step granularity.
		for i := len(result.Steps) - 1; i >= 0; i-- {
			if result.Steps[i].Status == "failed" {
				result.Steps[i].Status = "interrupted"
				break
			}
		}
		result.Status = "interrupted"
		result.Error = ctx.Err().Error()
		return result, runErr
	}
	if runErr != nil {
		result.Status = "failed"
		result.Error = runErr.Error()
		return result, runErr
	}

	// Emit bundle (mode: found only).
	enodeProvider := bundle.NewBesuEnodeProvider(deps.BesuRPCURL, nil)
	bundleInput := bundle.BundleInput{
		Manifest:      m,
		DataDir:       m.Spec.Node.DataDir,
		OutputDir:     in.OutputDir,
		EnodeProvider: enodeProvider,
	}
	_, bundleErr := fns.emitBundle(ctx, bundleInput)
	if bundleErr != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("bundle emission failed: %v", bundleErr)
		return result, bundleErr
	}

	bundlePath := filepath.Join("bundles", spokeID+".bundle.yaml")
	result.Status = "success"
	result.Bundle = &BundleRef{Path: bundlePath}
	return result, nil
}

// buildStepResults constructs the step report from orchestrator state.
func buildStepResults(stepOrder []string, state orchestrator.ProvisioningState) []StepResult {
	results := make([]StepResult, len(stepOrder))
	for i, name := range stepOrder {
		sr := StepResult{Name: name}
		found := false
		for _, s := range state.Steps {
			if s.Step == name {
				found = true
				switch s.Status {
				case "done":
					sr.Status = "skipped"
					sr.CompletedAt = s.CompletedAt
				case "failed":
					sr.Status = "failed"
				default:
					sr.Status = "pending"
				}
				break
			}
		}
		if !found {
			sr.Status = "pending"
		}
		results[i] = sr
	}
	return results
}

// pendingSteps returns 10 StepResults all with status "pending".
func pendingSteps() []StepResult {
	results := make([]StepResult, len(orchestrator.CanonicalStepOrder))
	for i, name := range orchestrator.CanonicalStepOrder {
		results[i] = StepResult{Name: name, Status: "pending"}
	}
	return results
}

// resolveLocalProfileFromInput builds a LocalProfile from ApplyInput fields.
// The binary path is unknown at this layer; callers in main.go set in.BesuRPCURL
// and in.OutputDir from a fully resolved LocalProfile.
func resolveLocalProfileFromInput(in ApplyInput) LocalProfile {
	rpcPort := 0
	if in.Manifest.Spec.Node.RPC != nil {
		rpcPort = in.Manifest.Spec.Node.RPC.Port
	}
	p := LocalProfile{
		BesuRPCURL: in.BesuRPCURL,
		OutputDir:  in.OutputDir,
		PaladinCBURL: envOr("CBWEB3_PALADIN_CB_URL", "http://localhost:31648"),
		ScriptsDir:   envOr("CBWEB3_SCRIPTS_DIR", ""),
		ComposeTemplatePath: envOr("CBWEB3_COMPOSE_TEMPLATE", ""),
		PaladinConfigDir:    envOr("CBWEB3_PALADIN_CONFIG_DIR", ""),
	}
	if p.BesuRPCURL == "" && rpcPort > 0 {
		p.BesuRPCURL = fmt.Sprintf("http://localhost:%d", rpcPort)
	}
	if p.OutputDir == "" {
		p.OutputDir = filepath.Dir(in.Manifest.Spec.Node.DataDir)
	}
	return p
}


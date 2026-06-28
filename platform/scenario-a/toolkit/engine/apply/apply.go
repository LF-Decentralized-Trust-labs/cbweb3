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
	runJoin    func(ctx context.Context, m *manifest.Manifest, b *bundle.JoinBundle, deps orchestrator.JoinDeps) error
	loadBundle func(path string) (*bundle.JoinBundle, error)
}

func defaultRunnerFuncs() runnerFuncs {
	return runnerFuncs{
		runFound:   orchestrator.RunFound,
		emitBundle: bundle.EmitBundle,
		runJoin:    orchestrator.RunJoin,
		loadBundle: bundle.LoadBundle,
	}
}

// Run provisions the participant described by in.Manifest, dispatching by mode.
// Returns a fully populated ApplyResult even on error (partial state captured).
func Run(ctx context.Context, in ApplyInput) (ApplyResult, error) {
	return run(ctx, in, defaultRunnerFuncs())
}

func run(ctx context.Context, in ApplyInput, fns runnerFuncs) (ApplyResult, error) {
	switch in.Manifest.Spec.Mode {
	case "join":
		return runJoinMode(ctx, in, fns)
	case "found", "":
		return runFoundMode(ctx, in, fns)
	default:
		spokeID := in.Manifest.Spec.Spoke.ID
		return ApplyResult{
			Spoke:  spokeID,
			Mode:   in.Manifest.Spec.Mode,
			Status: "failed",
			Error:  fmt.Sprintf("unknown mode %q (accepted: found, join)", in.Manifest.Spec.Mode),
		}, fmt.Errorf("apply: unknown mode %q", in.Manifest.Spec.Mode)
	}
}

func runFoundMode(ctx context.Context, in ApplyInput, fns runnerFuncs) (ApplyResult, error) {
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
		CBEndpoint:    m.Spec.CBEndpoint,
	}
	emittedBundle, bundleErr := fns.emitBundle(ctx, bundleInput)
	if bundleErr != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("bundle emission failed: %v", bundleErr)
		return result, bundleErr
	}
	// Fail fast if the emitted bundle is not usable for a subsequent mode:join
	// (e.g. validators could not be derived or cbEndpoint is absent), so the
	// fault surfaces here rather than when a commercial bank later tries to join.
	if err := bundle.ValidateForJoin(emittedBundle); err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("emitted bundle is not join-usable: %v", err)
		return result, err
	}

	bundlePath := filepath.Join("bundles", spokeID+".bundle.yaml")
	result.Status = "success"
	result.Bundle = &BundleRef{Path: bundlePath}
	return result, nil
}

// runJoinMode executes the mode:join flow: load + validate the bundle, resolve
// JoinDeps, run the 9-step engine, and build the step report.
func runJoinMode(ctx context.Context, in ApplyInput, fns runnerFuncs) (ApplyResult, error) {
	m := in.Manifest
	spokeID := m.Spec.Spoke.ID

	result := ApplyResult{
		Spoke:  spokeID,
		Mode:   m.Spec.Mode,
		DryRun: false,
	}

	if err := os.MkdirAll(m.Spec.Node.DataDir, 0o755); err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("create data dir: %v", err)
		result.Steps = pendingJoinSteps()
		return result, err
	}

	// Load and validate the join bundle.
	b, err := fns.loadBundle(in.JoinBundlePath)
	if err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("load join bundle: %v", err)
		result.Steps = pendingJoinSteps()
		return result, err
	}
	if err := bundle.ValidateForJoin(b); err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("invalid join bundle: %v", err)
		result.Steps = pendingJoinSteps()
		return result, err
	}

	deps, err := ResolveJoinDeps(m, resolveLocalProfileFromInput(in))
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		result.Steps = pendingJoinSteps()
		return result, err
	}

	runErr := fns.runJoin(ctx, m, b, deps)

	state, _ := orchestrator.LoadState(m.Spec.Node.DataDir)
	result.Steps = buildStepResults(orchestrator.CanonicalJoinStepOrder, state)

	if ctx.Err() != nil {
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

	result.Status = "success"
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

// pendingJoinSteps returns the 9 mode:join StepResults all with status "pending".
func pendingJoinSteps() []StepResult {
	results := make([]StepResult, len(orchestrator.CanonicalJoinStepOrder))
	for i, name := range orchestrator.CanonicalJoinStepOrder {
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
		CommercialBankComposePath: firstNonEmpty(in.CommercialBankComposePath, envOr("CBWEB3_COMMERCIAL_BANK_COMPOSE", "")),
		BackendComposePath:        firstNonEmpty(in.BackendComposePath, envOr("CBWEB3_BACKEND_COMPOSE", "")),
	}
	if p.BesuRPCURL == "" && rpcPort > 0 {
		p.BesuRPCURL = fmt.Sprintf("http://localhost:%d", rpcPort)
	}
	if p.OutputDir == "" {
		p.OutputDir = filepath.Dir(in.Manifest.Spec.Node.DataDir)
	}
	return p
}


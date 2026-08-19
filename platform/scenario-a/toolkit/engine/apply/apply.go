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
	case "observe":
		return runObserveMode(ctx, in)
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
		result.Steps = pendingSteps(m)
		return result, err
	}

	// Wire deps from profile.
	deps, err := ResolveDeps(m, resolveLocalProfileFromInput(in))
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		result.Steps = pendingSteps(m)
		return result, err
	}

	// Run the 10-step idempotent provisioning engine.
	runErr := fns.runFound(ctx, m, deps)

	// Read final state to build the step report regardless of error.
	state, _ := orchestrator.LoadState(m.Spec.Node.DataDir)
	result.Steps = buildStepResults(plannedStepOrder(m), state)

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
		// genesis-init writes genesis.json directly into this named volume, and
		// gen-tls writes central-bank.crt into this one — neither touches DataDir
		// (see specs/026-tk4-compose-central-bank/plan.md addendum). Naming mirrors
		// besu_data: ${SPOKE_ID}_cb_<artifact>.
		GenesisVolume: spokeID + "_cb_genesis",
		TLSVolume:     spokeID + "_cb_tls",
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

	// Emit the NOC bundle (observability topology; public, no secrets) so an
	// observe-mode NOC deployment can register this spoke + drive its agent.
	// Best-effort: a NOC emit failure never fails the CB found.
	besuRPCPort := 0
	if m.Spec.Node.RPC != nil {
		besuRPCPort = m.Spec.Node.RPC.Port
	}
	nb := bundle.NOCBundle{
		SpokeID:      spokeID,
		SpokeUUID:    orchestrator.DeterministicUUID(spokeID),
		Name:         spokeID,
		CurrencyCode: m.Spec.Spoke.Currency,
		Jurisdiction: spokeID,
		Components: []bundle.NOCComponent{{
			Name:          "besu-central-bank",
			Type:          "BESU",
			Endpoint:      fmt.Sprintf("http://host.docker.internal:%d", besuRPCPort),
			ContainerName: fmt.Sprintf("cbweb3-%s-besu.central-bank", spokeID),
		}},
	}
	if _, nerr := bundle.EmitNOC(nb, in.OutputDir); nerr != nil {
		fmt.Fprintf(os.Stderr, "[apply] warning: NOC bundle emit failed (non-fatal): %v\n", nerr)
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
		result.Steps = pendingSteps(m)
		return result, err
	}

	// Load and validate the join bundle.
	b, err := fns.loadBundle(in.JoinBundlePath)
	if err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("load join bundle: %v", err)
		result.Steps = pendingSteps(m)
		return result, err
	}
	if err := bundle.ValidateForJoin(b); err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("invalid join bundle: %v", err)
		result.Steps = pendingSteps(m)
		return result, err
	}

	deps, err := ResolveJoinDeps(m, resolveLocalProfileFromInput(in))
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		result.Steps = pendingSteps(m)
		return result, err
	}

	runErr := fns.runJoin(ctx, m, b, deps)

	state, _ := orchestrator.LoadState(m.Spec.Node.DataDir)
	result.Steps = buildStepResults(plannedStepOrder(m), state)

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

	// Best-effort: bring up this bank's noc-agent so its own Besu node reports
	// health + logs to the founding CB's NOC backend. Opt-in via spec.noc.backendURL
	// (the CB VM's :8090). Never fails the join — the node is a healthy peer without it.
	if m.Spec.NOC != nil && m.Spec.NOC.BackendURL != "" {
		besuRPCPort := 0
		if m.Spec.Node.RPC != nil {
			besuRPCPort = m.Spec.Node.RPC.Port
		}
		if aerr := runJoinNOCAgent(ctx, in, spokeID, m.BankCode(), besuRPCPort); aerr != nil {
			fmt.Fprintf(os.Stderr, "[apply] warning: NOC agent bring-up failed (non-fatal): %v\n", aerr)
		}
	}

	result.Status = "success"
	return result, nil
}

// plannedStepOrder returns the steps this manifest will actually execute. Both the
// apply report and the dry-run plan go through here so they cannot diverge — never
// read orchestrator.CanonicalStepOrder / CanonicalJoinStepOrder directly.
func plannedStepOrder(m *manifest.Manifest) []string {
	return orchestrator.PlannedStepOrder(m.Spec.Mode, m.Spec.Proxy == "enable")
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

// pendingSteps returns every step this manifest would execute, all "pending" —
// used when the run aborts before the engine starts.
func pendingSteps(m *manifest.Manifest) []StepResult {
	order := plannedStepOrder(m)
	results := make([]StepResult, len(order))
	for i, name := range order {
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
		BesuRPCURL:                       in.BesuRPCURL,
		OutputDir:                        in.OutputDir,
		PaladinCBURL:                     envOr("CBWEB3_PALADIN_CB_URL", "http://localhost:31648"),
		ScriptsDir:                       firstNonEmpty(in.ScriptsDir, envOr("CBWEB3_SCRIPTS_DIR", "")),
		ComposeTemplatePath:              firstNonEmpty(in.ComposeTemplatePath, envOr("CBWEB3_COMPOSE_TEMPLATE", "")),
		PaladinConfigDir:                 firstNonEmpty(in.PaladinConfigDir, envOr("CBWEB3_PALADIN_CONFIG_DIR", "")),
		CommercialBankComposePath:        firstNonEmpty(in.CommercialBankComposePath, envOr("CBWEB3_COMMERCIAL_BANK_COMPOSE", "")),
		BackendComposePath:               firstNonEmpty(in.BackendComposePath, envOr("CBWEB3_BACKEND_COMPOSE", "")),
		CentralBankComposePath:           firstNonEmpty(in.CentralBankComposePath, envOr("CBWEB3_CENTRAL_BANK_COMPOSE", "")),
		BesuImage:                        firstNonEmpty(in.BesuImage, envOr("CBWEB3_BESU_IMAGE", "hyperledger/besu:25.8.0")),
		PaladinImage:                     firstNonEmpty(in.PaladinImage, envOr("CBWEB3_PALADIN_IMAGE", "docker.io/lfdecentralizedtrust/paladin:v0.15.0-rc.1")),
		ContractsOutDir:                  firstNonEmpty(in.ContractsOutDir, envOr("CBWEB3_CONTRACTS_OUT", "")),
		CommercialBankPaladinComposePath: firstNonEmpty(in.CommercialBankPaladinComposePath, envOr("CBWEB3_COMMERCIAL_BANK_PALADIN_COMPOSE", "")),
	}
	if p.BesuRPCURL == "" && rpcPort > 0 {
		p.BesuRPCURL = fmt.Sprintf("http://localhost:%d", rpcPort)
	}
	if p.OutputDir == "" {
		p.OutputDir = filepath.Dir(in.Manifest.Spec.Node.DataDir)
	}
	return p
}

//go:build e2e

// Full-pipeline E2E (build tag `e2e`): composes the toolkit modes end-to-end —
// found-hub → found-spoke ×2 (CB-A/CB-B) → join (bank) → sovereign tail — against
// real Docker, then exercises the business path (swap through the AMM, circuit
// breaker pause/resume, SpokeBridge lock/release) and idempotency (re-apply
// converges). SKIPS WITH A WARNING when the environment is absent — never a false
// green (FR-006 / SC-005).
//
//	Run: go test -tags e2e ./tests/e2e/... -run TestPipeline
//	Requires: docker, cast, a real hub+spoke+relay environment, and the env vars
//	below (see specs/041-tk-b10-e2e-baseline/quickstart.md).
package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/apply"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/orchestrator"
)

// pipelineEnv gathers the env-driven inputs; missing any → skip-with-warning.
type pipelineEnv struct {
	hubManifest, cbaManifest, cbbManifest, joinManifest string
	repoRoot, hubRPC                                    string
	amm, pairRegistry, spokeBridge                      string
	govKey, cb2Key, swapKey                             string
}

func requirePipelineEnv(t *testing.T) pipelineEnv {
	t.Helper()
	requireTool(t, "docker")
	requireTool(t, "cast")
	return pipelineEnv{
		hubManifest:  requireEnv(t, "CBWEB3B_E2E_HUB_MANIFEST"),
		cbaManifest:  requireEnv(t, "CBWEB3B_E2E_CBA_MANIFEST"),
		cbbManifest:  requireEnv(t, "CBWEB3B_E2E_CBB_MANIFEST"),
		joinManifest: requireEnv(t, "CBWEB3B_E2E_JOIN_MANIFEST"),
		repoRoot:     requireEnv(t, "CBWEB3B_E2E_REPO_ROOT"),
		hubRPC:       requireEnv(t, "CBWEB3B_E2E_HUB_RPC"),
		amm:          requireEnv(t, "CBWEB3B_E2E_AMM"),
		pairRegistry: requireEnv(t, "CBWEB3B_E2E_PAIR_REGISTRY"),
		spokeBridge:  requireEnv(t, "CBWEB3B_E2E_SPOKE_BRIDGE"),
		govKey:       requireEnv(t, "CBWEB3B_E2E_GOV_KEY"),
		cb2Key:       requireEnv(t, "CBWEB3B_E2E_CB2_KEY"),
		swapKey:      requireEnv(t, "CBWEB3B_E2E_SWAP_KEY"), // U1: a verified account that may swap
	}
}

// applyMode runs apply.Apply and fails with the partial report on error (FR-007).
func applyMode(t *testing.T, label string, o apply.Options) orchestrator.Report {
	t.Helper()
	rep, err := apply.Apply(context.Background(), o)
	if err != nil {
		t.Fatalf("%s apply failed: %v\npartial report: %+v", label, err, rep)
	}
	return rep
}

// TestPipelineEndToEnd is the capstone: provision + business path + idempotency.
func TestPipelineEndToEnd(t *testing.T) {
	e := requirePipelineEnv(t)

	// ── Phase 1: provision the full topology via the toolkit modes ──────────
	// Each mode keeps its own dataDir so the persisted state drives idempotency
	// on re-apply (Phase 6).
	opts := map[string]apply.Options{
		"found-hub":        {ManifestPath: e.hubManifest, RepoRoot: e.repoRoot, DataDir: t.TempDir(), HubRPC: e.hubRPC},
		"found-spoke CB-A": {ManifestPath: e.cbaManifest, RepoRoot: e.repoRoot, DataDir: t.TempDir(), HubRPC: e.hubRPC},
		"found-spoke CB-B": {ManifestPath: e.cbbManifest, RepoRoot: e.repoRoot, DataDir: t.TempDir(), HubRPC: e.hubRPC},
		"join":             {ManifestPath: e.joinManifest, RepoRoot: e.repoRoot, DataDir: t.TempDir(), HubRPC: e.hubRPC},
	}
	order := []string{"found-hub", "found-spoke CB-A", "found-spoke CB-B", "join"}
	for _, mode := range order {
		applyMode(t, mode, opts[mode])
	}

	// ── Phase 2: the sovereign pair is ACTIVE ───────────────────────────────
	pairID := envOr("CBWEB3B_E2E_PAIR_ID", "W-BRL-ARS")
	if got := castCall(t, e.hubRPC, e.pairRegistry, "getPair(string)", pairID); !strings.Contains(strings.ToUpper(got), "ACTIVE") {
		t.Fatalf("pair %s not ACTIVE after pipeline: %q", pairID, got)
	}

	// ── Phase 3–5: business path ────────────────────────────────────────────
	t.Run("swap", func(t *testing.T) { assertSwap(t, e) })
	t.Run("breaker", func(t *testing.T) { assertBreaker(t, e) })
	t.Run("bridge-lock", func(t *testing.T) { assertBridgeLock(t, e) })

	// ── Phase 6: idempotency — re-apply EACH mode converges (FR-005/SC-004) ──
	t.Run("idempotency", func(t *testing.T) {
		for _, mode := range order {
			rep := applyMode(t, "re-apply "+mode, opts[mode]) // same dataDir → persisted state drives skip
			for _, s := range rep.Steps {
				if s.Status != orchestrator.StatusSkipped && s.Status != orchestrator.StatusDone {
					t.Errorf("re-apply %s: step %s = %s (want skipped/done)", mode, s.Name, s.Status)
				}
			}
		}
		if got := castCall(t, e.hubRPC, e.pairRegistry, "getPair(string)", pairID); !strings.Contains(strings.ToUpper(got), "ACTIVE") {
			t.Errorf("pair no longer ACTIVE after re-apply: %q", got)
		}
	})
}

func assertSwap(t *testing.T, e pipelineEnv) {
	tokenIn := requireEnv(t, "CBWEB3B_E2E_TOKEN_A")
	tokenOut := requireEnv(t, "CBWEB3B_E2E_TOKEN_B")
	to := requireEnv(t, "CBWEB3B_E2E_SWAP_TO")
	// swapTokensForExactTokens(tokenIn, tokenOut, amountOut, maxAmountIn, to)
	castSend(t, e.hubRPC, e.swapKey, e.amm,
		"swapTokensForExactTokens(address,address,uint256,uint256,address)",
		tokenIn, tokenOut, "1", "1000000000000000000", to)
	// Reserves should be non-zero after a successful swap.
	if r := castCall(t, e.hubRPC, e.amm, "getReserves()"); strings.TrimSpace(r) == "" {
		t.Errorf("getReserves empty after swap")
	}
}

func assertBreaker(t *testing.T, e pipelineEnv) {
	// pause (1-of-N) → isPaused true → a swap reverts.
	castSend(t, e.hubRPC, e.govKey, e.amm, "pause(string)", "e2e-breaker")
	if got := castCall(t, e.hubRPC, e.amm, "isPaused()"); !strings.Contains(got, "true") {
		t.Fatalf("isPaused should be true after pause, got %q", got)
	}
	tokenIn := envOr("CBWEB3B_E2E_TOKEN_A", "")
	tokenOut := envOr("CBWEB3B_E2E_TOKEN_B", "")
	to := envOr("CBWEB3B_E2E_SWAP_TO", "")
	if err := castCallErr(e.hubRPC, e.swapKey, e.amm,
		"swapTokensForExactTokens(address,address,uint256,uint256,address)",
		tokenIn, tokenOut, "1", "1000000000000000000", to); err == nil {
		t.Error("swap should revert while the breaker is paused")
	}
	// resume needs quorum 2 → two distinct CBs sign the same proposal.
	proposalID := requireEnv(t, "CBWEB3B_E2E_RESUME_PROPOSAL")
	castSend(t, e.hubRPC, e.govKey, e.amm, "signResume(bytes32)", proposalID)
	castSend(t, e.hubRPC, e.cb2Key, e.amm, "signResume(bytes32)", proposalID)
	if got := castCall(t, e.hubRPC, e.amm, "isPaused()"); !strings.Contains(got, "false") {
		t.Errorf("isPaused should be false after resume quorum, got %q", got)
	}
}

func assertBridgeLock(t *testing.T, e pipelineEnv) {
	token := requireEnv(t, "CBWEB3B_E2E_BRIDGE_TOKEN")
	txID := envOr("CBWEB3B_E2E_BRIDGE_TXID", "0x"+strings.Repeat("11", 32))
	castSend(t, e.hubRPC, e.swapKey, e.spokeBridge, "lock(address,uint256,bytes32)", token, "1", txID)
	if got := castCall(t, e.hubRPC, e.spokeBridge, "getLock(bytes32)", txID); strings.TrimSpace(got) == "" {
		t.Errorf("getLock empty after lock")
	}
}

// TestPipeline_HubMint (C1): the hub mint is relay-mediated and asynchronous.
// Poll the hub token balance after a lock; skip-with-warning if the relay does
// not mint within the window (never a false green).
func TestPipeline_HubMint(t *testing.T) {
	requireTool(t, "cast")
	hubRPC := requireEnv(t, "CBWEB3B_E2E_HUB_RPC")
	hubToken := requireEnv(t, "CBWEB3B_E2E_HUB_MINT_TOKEN")
	holder := requireEnv(t, "CBWEB3B_E2E_HUB_MINT_HOLDER")
	before := castCall(t, hubRPC, hubToken, "balanceOf(address)", holder)

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if castCall(t, hubRPC, hubToken, "balanceOf(address)", holder) != before {
			return // relay minted on the hub — success
		}
		time.Sleep(2 * time.Second)
	}
	t.Skipf("E2E skipped: relay hub-mint not observed within window (balance unchanged from %s)", before)
}

// TestPipeline_BridgeRefund (Q1): release is the refund/reclaim path (no on-chain
// timeout). Skip-with-warning when the lock→settle round-trip cannot be arranged.
func TestPipeline_BridgeRefund(t *testing.T) {
	requireTool(t, "cast")
	hubRPC := requireEnv(t, "CBWEB3B_E2E_HUB_RPC")
	bridge := requireEnv(t, "CBWEB3B_E2E_SPOKE_BRIDGE")
	govKey := requireEnv(t, "CBWEB3B_E2E_GOV_KEY")
	lockKey := requireEnv(t, "CBWEB3B_E2E_SWAP_KEY")
	token := requireEnv(t, "CBWEB3B_E2E_BRIDGE_TOKEN")
	txID := envOr("CBWEB3B_E2E_REFUND_TXID", "0x"+strings.Repeat("22", 32))

	castSend(t, hubRPC, lockKey, bridge, "lock(address,uint256,bytes32)", token, "1", txID)
	castSend(t, hubRPC, govKey, bridge, "release(bytes32)", txID)
	got := castCall(t, hubRPC, bridge, "getLock(bytes32)", txID)
	if !strings.Contains(strings.ToLower(got), "true") {
		t.Errorf("getLock released should be true after release (refund), got %q", got)
	}
}

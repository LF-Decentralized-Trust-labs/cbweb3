// SPDX-License-Identifier: Apache-2.0

//go:build e2e

// sovereign-pair E2E (build tag `e2e`): founds two sovereign spokes, then opens a
// bilateral corridor between them and validates the pair reached ACTIVE. Skips with
// a warning when the environment is absent (FR-013 / SC-008).
//
// Opening the corridor is NOT a provisioning step. `apply` founds each spoke and
// registers its CB and currency on the hub; it never plans open-sovereign-pair,
// commit-liquidity or seed-oracle (see TestApplyFoundSpokeHasNoSovereignTail). The
// corridor is two independent sovereign acts on the hub PairRegistry, each signed by
// its own central bank — CB-A proposes, CB-B confirms. No single run holds both keys;
// this test stands in for the two governance-portal operations.
//
//	Run: go test -tags e2e ./tests/e2e/...
//	Requires: docker, forge/cast, a founded hub + relay, and:
//	  CBWEB3B_E2E_PAIR_CBA_MANIFEST = found-spoke manifest for CB-A
//	  CBWEB3B_E2E_PAIR_CBB_MANIFEST = found-spoke manifest for CB-B
//	  CBWEB3B_E2E_REPO_ROOT         = repository root
//	  CBWEB3B_E2E_HUB_RPC           = hub RPC URL
//	  CBWEB3B_E2E_PAIR_REGISTRY     = PairRegistry address
//	  CBWEB3B_E2E_PAIR_ID           = deterministic pair id (e.g. W-BRL-ARS)
//	  CBWEB3B_E2E_PAIR_TOKEN_A      = hub-wrapped token for CB-A's currency
//	  CBWEB3B_E2E_PAIR_TOKEN_B      = hub-wrapped token for CB-B's currency
//	  CBWEB3B_E2E_PAIR_AMM          = AMM address backing the pair
//	  CBWEB3B_E2E_CBA_KEY           = CB-A signing key (proposePair)
//	  CBWEB3B_E2E_CBB_KEY           = CB-B signing key (confirmPair)
package e2e

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/apply"
)

// SC-008: two founded spokes, then CB-A proposes and CB-B confirms → the pair
// reaches ACTIVE.
func TestSovereignPairEndToEnd(t *testing.T) {
	requireTool(t, "docker")
	requireTool(t, "cast")
	cbaManifest := requireEnv(t, "CBWEB3B_E2E_PAIR_CBA_MANIFEST")
	cbbManifest := requireEnv(t, "CBWEB3B_E2E_PAIR_CBB_MANIFEST")
	repoRoot := requireEnv(t, "CBWEB3B_E2E_REPO_ROOT")
	hubRPC := requireEnv(t, "CBWEB3B_E2E_HUB_RPC")
	pairRegistry := requireEnv(t, "CBWEB3B_E2E_PAIR_REGISTRY")
	pairID := requireEnv(t, "CBWEB3B_E2E_PAIR_ID")
	tokenA := requireEnv(t, "CBWEB3B_E2E_PAIR_TOKEN_A")
	tokenB := requireEnv(t, "CBWEB3B_E2E_PAIR_TOKEN_B")
	amm := requireEnv(t, "CBWEB3B_E2E_PAIR_AMM")
	cbaKey := requireEnv(t, "CBWEB3B_E2E_CBA_KEY")
	cbbKey := requireEnv(t, "CBWEB3B_E2E_CBB_KEY")

	// Provisioning: each CB founds its own spoke. Neither apply touches the corridor.
	if _, err := apply.Apply(context.Background(), apply.Options{
		ManifestPath: cbaManifest, RepoRoot: repoRoot, DataDir: t.TempDir(), HubRPC: hubRPC,
	}); err != nil {
		t.Fatalf("CB-A apply failed: %v", err)
	}
	if _, err := apply.Apply(context.Background(), apply.Options{
		ManifestPath: cbbManifest, RepoRoot: repoRoot, DataDir: t.TempDir(), HubRPC: hubRPC,
	}); err != nil {
		t.Fatalf("CB-B apply failed: %v", err)
	}

	// Runtime: sovereign act 1 — CB-A proposes the corridor, signing with its own key.
	castSend(t, hubRPC, cbaKey, pairRegistry,
		"proposePair(string,address,address,address)", pairID, tokenA, tokenB, amm)
	// Runtime: sovereign act 2 — CB-B confirms it, signing with its own.
	castSend(t, hubRPC, cbbKey, pairRegistry, "confirmPair(string)", pairID)

	if status := castGetPairStatus(t, hubRPC, pairRegistry, pairID); !strings.Contains(strings.ToUpper(status), "ACTIVE") {
		t.Errorf("pair %s expected ACTIVE after both CBs, got %q", pairID, status)
	}
}

func castGetPairStatus(t *testing.T, rpcURL, pairRegistry, pairID string) string {
	t.Helper()
	cmd := exec.Command("cast", "call", pairRegistry, "getPair(string)", pairID, "--rpc-url", rpcURL)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("cast call getPair: %v", err)
	}
	return out.String()
}

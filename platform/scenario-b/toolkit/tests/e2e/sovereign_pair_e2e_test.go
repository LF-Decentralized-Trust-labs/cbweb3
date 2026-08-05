// SPDX-License-Identifier: Apache-2.0

//go:build e2e

// sovereign-pair E2E (build tag `e2e`): opens a bilateral corridor between two
// founded spokes with Docker + Foundry + a running relay, then validates the
// pair reached ACTIVE. Strict sovereignty: CB-A proposes, CB-B confirms — two
// separate found-spoke applies. Skips with a warning when the environment is
// absent (FR-013 / SC-008).
//
//	Run: go test -tags e2e ./tests/e2e/...
//	Requires: docker, forge/cast, a founded hub + relay, and:
//	  CBWEB3B_E2E_PAIR_CBA_MANIFEST = found-spoke manifest for CB-A (with spec.pair)
//	  CBWEB3B_E2E_PAIR_CBB_MANIFEST = found-spoke manifest for CB-B (same spec.pair)
//	  CBWEB3B_E2E_REPO_ROOT         = repository root
//	  CBWEB3B_E2E_HUB_RPC           = hub RPC URL
//	  CBWEB3B_E2E_PAIR_REGISTRY     = PairRegistry address
//	  CBWEB3B_E2E_PAIR_ID           = deterministic pair id (e.g. W-BRL-ARS)
package e2e

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/apply"
)

// SC-008: CB-A proposes, CB-B confirms → the pair reaches ACTIVE.
func TestSovereignPairEndToEnd(t *testing.T) {
	requireTool(t, "docker")
	requireTool(t, "cast")
	cbaManifest := requireEnv(t, "CBWEB3B_E2E_PAIR_CBA_MANIFEST")
	cbbManifest := requireEnv(t, "CBWEB3B_E2E_PAIR_CBB_MANIFEST")
	repoRoot := requireEnv(t, "CBWEB3B_E2E_REPO_ROOT")
	hubRPC := requireEnv(t, "CBWEB3B_E2E_HUB_RPC")
	pairRegistry := requireEnv(t, "CBWEB3B_E2E_PAIR_REGISTRY")
	pairID := requireEnv(t, "CBWEB3B_E2E_PAIR_ID")

	// CB-A: found-spoke + open-sovereign-pair (scaffolding + proposePair).
	if _, err := apply.Apply(context.Background(), apply.Options{
		ManifestPath: cbaManifest, RepoRoot: repoRoot, DataDir: t.TempDir(), HubRPC: hubRPC,
	}); err != nil {
		t.Fatalf("CB-A apply failed: %v", err)
	}
	// CB-B: found-spoke + confirmPair → ACTIVE.
	if _, err := apply.Apply(context.Background(), apply.Options{
		ManifestPath: cbbManifest, RepoRoot: repoRoot, DataDir: t.TempDir(), HubRPC: hubRPC,
	}); err != nil {
		t.Fatalf("CB-B apply failed: %v", err)
	}

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

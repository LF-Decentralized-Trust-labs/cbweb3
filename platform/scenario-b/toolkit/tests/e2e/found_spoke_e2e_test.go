// SPDX-License-Identifier: Apache-2.0

//go:build e2e

// found-spoke E2E (build tag `e2e`): funds a spoke against a founded hub with
// Docker + Foundry + Besu and validates the spoke bundle. Skips with a warning
// when the environment is absent (FR-014 / SC-009).
//
//	Run: go test -tags e2e ./tests/e2e/...
//	Requires: docker, forge, and:
//	  CBWEB3B_E2E_SPOKE_MANIFEST = path to a found-spoke manifest (with hubBundleRef)
//	  CBWEB3B_E2E_REPO_ROOT      = repository root
//	  CBWEB3B_E2E_SPOKE_RPC      = spoke RPC URL
package e2e

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/apply"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/bundle"
)

// SC-009: apply found-spoke for real; the spoke bundle validates and its
// contracts respond on the live spoke chain.
func TestFoundSpokeEndToEnd(t *testing.T) {
	requireTool(t, "docker")
	requireTool(t, "forge")
	manifestPath := requireEnv(t, "CBWEB3B_E2E_SPOKE_MANIFEST")
	repoRoot := requireEnv(t, "CBWEB3B_E2E_REPO_ROOT")
	spokeRPC := requireEnv(t, "CBWEB3B_E2E_SPOKE_RPC")

	dataDir := t.TempDir()
	rep, err := apply.Apply(context.Background(), apply.Options{
		ManifestPath: manifestPath,
		RepoRoot:     repoRoot,
		DataDir:      dataDir,
		OutDir:       dataDir,
		SpokeRPC:     spokeRPC,
		GatewayURL:   os.Getenv("CBWEB3B_E2E_SPOKE_GATEWAY"),
		Relay:        os.Getenv("CBWEB3B_E2E_RELAY"),
		DryRun:       false,
	})
	if err != nil {
		t.Fatalf("found-spoke apply failed: %v\nreport: %+v", err, rep)
	}

	// Find the emitted spoke bundle and validate it against the live chain.
	matches, _ := filepath.Glob(filepath.Join(dataDir, "bundles", "spoke-*.bundle.yaml"))
	if len(matches) == 0 {
		t.Fatal("no spoke bundle emitted")
	}
	b, err := bundle.LoadSpoke(matches[0])
	if err != nil {
		t.Fatalf("load spoke bundle: %v", err)
	}
	for _, name := range bundle.RequiredSpokeContracts {
		addr := b.Contracts[name]
		if addr == "" {
			t.Fatalf("bundle missing contract %s", name)
		}
		if !hasCode(t, spokeRPC, addr) {
			t.Errorf("spoke contract %s at %s has no code on-chain", name, addr)
		}
	}
	if b.Enode == "" || b.Genesis == "" {
		t.Fatal("spoke bundle must carry enode + genesis for join")
	}
}

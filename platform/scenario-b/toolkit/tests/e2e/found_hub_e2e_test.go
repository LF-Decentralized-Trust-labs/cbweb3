// SPDX-License-Identifier: Apache-2.0

//go:build e2e

// Package e2e holds the end-to-end suite for the toolkit (build tag `e2e`, so
// it does not run in the default `go test ./...`). It funds the hub for real
// with Docker + Foundry + Besu and validates the hub bundle against the live
// chain. It SKIPS WITH A WARNING when the environment is absent — never a false
// green (FR-015 / SC-009).
//
//	Run: go test -tags e2e ./tests/e2e/...
//	Requires: docker, forge, hyperledger/besu image, and:
//	  CBWEB3B_E2E_MANIFEST = path to a complete found-hub manifest
//	  CBWEB3B_E2E_REPO_ROOT = repository root (for contracts/templates)
//	  CBWEB3B_E2E_HUB_RPC   = hub RPC URL
package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/apply"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/bundle"
)

func requireTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("E2E skipped: %q not found in PATH", name)
	}
}

func requireEnv(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Skipf("E2E skipped: %s not set (provide a real found-hub environment)", key)
	}
	return v
}

// SC-009: apply found-hub for real, then the hub bundle addresses must respond
// on the live chain.
func TestFoundHubEndToEnd(t *testing.T) {
	requireTool(t, "docker")
	requireTool(t, "forge")
	manifestPath := requireEnv(t, "CBWEB3B_E2E_MANIFEST")
	repoRoot := requireEnv(t, "CBWEB3B_E2E_REPO_ROOT")
	hubRPC := requireEnv(t, "CBWEB3B_E2E_HUB_RPC")

	dataDir := t.TempDir()
	rep, err := apply.Apply(context.Background(), apply.Options{
		ManifestPath: manifestPath,
		RepoRoot:     repoRoot,
		DataDir:      dataDir,
		OutDir:       dataDir,
		HubRPC:       hubRPC,
		DryRun:       false,
	})
	if err != nil {
		t.Fatalf("found-hub apply failed: %v\nreport: %+v", err, rep)
	}

	b, err := bundle.LoadHub(filepath.Join(dataDir, "bundles", "hub.bundle.yaml"))
	if err != nil {
		t.Fatalf("load hub bundle: %v", err)
	}
	// Bundle carries all required contracts; each should have on-chain code.
	for _, name := range bundle.RequiredContracts {
		addr := b.Contracts[name]
		if addr == "" {
			t.Fatalf("bundle missing contract %s", name)
		}
		if !hasCode(t, hubRPC, addr) {
			t.Errorf("contract %s at %s has no code on-chain", name, addr)
		}
	}
}

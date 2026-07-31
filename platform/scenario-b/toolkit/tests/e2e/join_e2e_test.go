//go:build e2e

// join E2E (build tag `e2e`): a commercial bank joins a founded spoke with
// Docker + Besu and we validate the node synced (non-validating full node) and
// the CSR was generated locally. Skips with a warning when the environment is
// absent (FR-014 / SC-007).
//
//	Run: go test -tags e2e ./tests/e2e/...
//	Requires: docker, and:
//	  CBWEB3B_E2E_JOIN_MANIFEST = path to a join manifest (with joinBundleRef)
//	  CBWEB3B_E2E_REPO_ROOT     = repository root
//	  CBWEB3B_E2E_BANK_RPC      = the bank node's RPC URL
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/apply"
)

// SC-007: apply join for real; the bank node syncs and the CSR is generated.
func TestJoinEndToEnd(t *testing.T) {
	requireTool(t, "docker")
	manifestPath := requireEnv(t, "CBWEB3B_E2E_JOIN_MANIFEST")
	repoRoot := requireEnv(t, "CBWEB3B_E2E_REPO_ROOT")
	bankRPC := requireEnv(t, "CBWEB3B_E2E_BANK_RPC")

	dataDir := t.TempDir()
	rep, err := apply.Apply(context.Background(), apply.Options{
		ManifestPath: manifestPath,
		RepoRoot:     repoRoot,
		DataDir:      dataDir,
		OutDir:       dataDir,
		SpokeRPC:     bankRPC, // wait-sync gate targets the bank's own node
		DryRun:       false,
	})
	if err != nil {
		t.Fatalf("join apply failed: %v\nreport: %+v", err, rep)
	}

	// SC-003: the node is synced (eth_syncing == false).
	if syncing := ethSyncingRaw(t, bankRPC); syncing {
		t.Errorf("bank node still syncing after join")
	}

	// SC-005: CSR generated locally; key present; zero CA material.
	pkiDir := filepath.Join(dataDir, "pki")
	entries, err := os.ReadDir(pkiDir)
	if err != nil {
		t.Fatalf("pki dir not created: %v", err)
	}
	var haveKey, haveCSR bool
	for _, e := range entries {
		switch {
		case strings.HasSuffix(e.Name(), ".key"):
			haveKey = true
		case strings.HasSuffix(e.Name(), ".csr"):
			haveCSR = true
		case strings.Contains(e.Name(), "-ca."):
			t.Errorf("join must not produce CA material, found %s", e.Name())
		}
	}
	if !haveKey || !haveCSR {
		t.Fatalf("gen-csr did not produce key+csr in %s", pkiDir)
	}
}

// ethSyncingRaw returns whether the node reports syncing (eth_syncing != false).
func ethSyncingRaw(t *testing.T, rpcURL string) bool {
	t.Helper()
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"eth_syncing","params":[]}`)
	resp, err := http.Post(rpcURL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("eth_syncing: %v", err)
	}
	defer resp.Body.Close()
	var out struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode eth_syncing: %v", err)
	}
	return strings.TrimSpace(string(out.Result)) != "false"
}

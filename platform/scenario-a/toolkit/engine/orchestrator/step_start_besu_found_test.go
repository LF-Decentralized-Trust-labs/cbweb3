// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newTestStartBesuFoundStep builds a *startBesuFoundStep with the given RPC URL
// and data dir, suitable for unit tests (no Docker required for Check/scaffold).
func newTestStartBesuFoundStep(rpcURL, dataDir string, chainID int) *startBesuFoundStep {
	return &startBesuFoundStep{
		spokeID:        "spoke-test",
		chainID:        chainID,
		dataDir:        dataDir,
		composePath:    "/nonexistent/central-bank/docker-compose.yaml",
		besuRPCURL:     rpcURL,
		advertisedHost: "cbweb3-spoke-test-besu.central-bank-test",
		besuImage:      "hyperledger/besu:25.8.0",
		rpcPort:        8645,
		wsPort:         8655,
		p2pPort:        31303,
		hostUID:        1000,
		hostGID:        1000,
		healthTimeout:  2 * time.Second,
		healthInterval: 100 * time.Millisecond,
		httpClient:     sharedJoinHTTPClient,
	}
}

func TestStartBesuFoundStep_Check_NodeUp_True(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"jsonrpc":"2.0","id":1,"result":"0x10"}`)
	}))
	defer srv.Close()

	s := newTestStartBesuFoundStep(srv.URL, t.TempDir(), 1337)
	done, err := s.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("Check should return true when eth_blockNumber responds")
	}
}

func TestStartBesuFoundStep_Check_NodeDown_False(t *testing.T) {
	s := newTestStartBesuFoundStep("http://127.0.0.1:1", t.TempDir(), 1337)
	done, err := s.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("Check should return false when no node is reachable")
	}
}

func TestStartBesuFoundStep_Scaffold_RendersQBFTWithChainID(t *testing.T) {
	dataDir := t.TempDir()
	s := newTestStartBesuFoundStep("http://127.0.0.1:1", dataDir, 1338)

	if err := s.scaffold(); err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	// Required directories created.
	for _, sub := range []string{"config", "genesis", filepath.Join("nodes", "central-bank", "data")} {
		if _, err := os.Stat(filepath.Join(dataDir, sub)); err != nil {
			t.Errorf("expected dir %q to exist: %v", sub, err)
		}
	}

	// qbftConfigFile.json rendered with the manifest chainId.
	qbftPath := filepath.Join(dataDir, "config", "qbftConfigFile.json")
	data, err := os.ReadFile(qbftPath)
	if err != nil {
		t.Fatalf("read qbft config: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("qbft config is not valid JSON: %v", err)
	}
	genesisCfg := doc["genesis"].(map[string]any)["config"].(map[string]any)
	if got := genesisCfg["chainId"].(float64); int(got) != 1338 {
		t.Errorf("chainId = %v; want 1338", got)
	}
	if _, ok := genesisCfg["qbft"]; !ok {
		t.Error("qbft config missing genesis.config.qbft block")
	}
}

func TestStartBesuFoundStep_Scaffold_PreservesExistingQBFT(t *testing.T) {
	dataDir := t.TempDir()
	s := newTestStartBesuFoundStep("http://127.0.0.1:1", dataDir, 1337)

	// Pre-existing custom config must NOT be overwritten (non-destructive).
	configDir := filepath.Join(dataDir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	custom := []byte(`{"custom":true}`)
	if err := os.WriteFile(filepath.Join(configDir, "qbftConfigFile.json"), custom, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := s.scaffold(); err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(configDir, "qbftConfigFile.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(custom) {
		t.Errorf("scaffold overwrote existing qbftConfigFile.json: got %q", got)
	}
}

func TestStartBesuFoundStep_ComposeEnv_NoBootnode(t *testing.T) {
	s := newTestStartBesuFoundStep("http://127.0.0.1:1", t.TempDir(), 1337)
	env := strings.Join(s.composeEnv(), "\n")

	for _, want := range []string{
		"SPOKE_ID=spoke-test",
		"BESU_IMAGE=hyperledger/besu:25.8.0",
		"BESU_RPC_PORT=8645",
		"BESU_WS_PORT=8655",
		"BESU_P2P_PORT=31303",
		"BESU_ADVERTISED_HOST=cbweb3-spoke-test-besu.central-bank-test",
		"HOST_UID=1000",
		"HOST_GID=1000",
	} {
		if !strings.Contains(env, want) {
			t.Errorf("composeEnv missing %q", want)
		}
	}

	// The central bank IS the bootnode — it must never receive BOOTNODE_ENODE.
	if strings.Contains(env, "BOOTNODE_ENODE=") {
		t.Error("found-mode start-besu must not set BOOTNODE_ENODE (it is the bootnode)")
	}
}

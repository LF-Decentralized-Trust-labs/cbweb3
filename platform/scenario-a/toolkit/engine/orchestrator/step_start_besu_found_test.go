// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestStartBesuFoundStep builds a *startBesuFoundStep with the given RPC URL,
// suitable for unit tests. Check/ComposeEnv tests need no Docker; Scaffold tests
// exercise the real cb_config named volume and are skipped if Docker is absent
// (see requireDocker in volumefs_test.go).
func newTestStartBesuFoundStep(rpcURL string, chainID int) *startBesuFoundStep {
	return &startBesuFoundStep{
		spokeID:        "spoke-test",
		chainID:        chainID,
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

	s := newTestStartBesuFoundStep(srv.URL, 1337)
	done, err := s.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !done {
		t.Error("Check should return true when eth_blockNumber responds")
	}
}

func TestStartBesuFoundStep_Check_NodeDown_False(t *testing.T) {
	s := newTestStartBesuFoundStep("http://127.0.0.1:1", 1337)
	done, err := s.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("Check should return false when no node is reachable")
	}
}

func TestStartBesuFoundStep_Scaffold_RendersQBFTWithChainID(t *testing.T) {
	requireDocker(t)
	s := newTestStartBesuFoundStep("http://127.0.0.1:1", 1338)
	s.spokeID = "spoke-test-scaffold-render"
	cleanupVolume(t, s.configVolume())

	if err := s.scaffold(context.Background()); err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	// qbftConfigFile.json seeded into the volume — no host filesystem involved
	// (deviation from the original SPOKE_DATA_DIR bind-mount design).
	data, err := readVolumeFile(context.Background(), s.configVolume(), "qbftConfigFile.json")
	if err != nil {
		t.Fatalf("read qbft config from volume: %v", err)
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

	// The deployer dev account must be pre-funded or deploy-contracts fails.
	alloc, ok := doc["genesis"].(map[string]any)["alloc"].(map[string]any)
	if !ok {
		t.Fatal("genesis.alloc missing or not an object")
	}
	deployer, ok := alloc["fe3b557e8fb62b89f4916b721be55ceb828dbd73"].(map[string]any)
	if !ok {
		t.Fatal("deployer account 0xfe3b557e… is not funded in genesis.alloc")
	}
	if deployer["balance"] == "" || deployer["balance"] == nil {
		t.Error("deployer account has no balance in genesis.alloc")
	}
}

func TestStartBesuFoundStep_Scaffold_PreservesExistingQBFT(t *testing.T) {
	requireDocker(t)
	s := newTestStartBesuFoundStep("http://127.0.0.1:1", 1337)
	s.spokeID = "spoke-test-scaffold-preserve"
	cleanupVolume(t, s.configVolume())

	// Pre-existing custom config must NOT be overwritten (non-destructive).
	custom := []byte(`{"custom":true}`)
	if err := writeVolumeFile(context.Background(), s.configVolume(), "qbftConfigFile.json", custom, "0644"); err != nil {
		t.Fatal(err)
	}

	if err := s.scaffold(context.Background()); err != nil {
		t.Fatalf("scaffold: %v", err)
	}

	got, err := readVolumeFile(context.Background(), s.configVolume(), "qbftConfigFile.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(custom) {
		t.Errorf("scaffold overwrote existing qbftConfigFile.json: got %q", got)
	}
}

func TestStartBesuFoundStep_ComposeEnv_NoBootnode(t *testing.T) {
	s := newTestStartBesuFoundStep("http://127.0.0.1:1", 1337)
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

	// SPOKE_DATA_DIR is no longer consumed by the central-bank compose template
	// (config/genesis moved to named volumes cb_config/cb_genesis) — must not be set.
	if strings.Contains(env, "SPOKE_DATA_DIR=") {
		t.Error("composeEnv must not set SPOKE_DATA_DIR (unused since the volume migration)")
	}

	// The central bank IS the bootnode — it must never receive BOOTNODE_ENODE.
	if strings.Contains(env, "BOOTNODE_ENODE=") {
		t.Error("found-mode start-besu must not set BOOTNODE_ENODE (it is the bootnode)")
	}
}

// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// defaultBesuImage is the pinned Besu image used for the spoke bootnode when the
// deps do not override it. Matches the project-pinned Hyperledger Besu version.
const defaultBesuImage = "hyperledger/besu:25.8.0"

// startBesuFoundStep brings up the central-bank Besu node (the spoke bootnode)
// for mode:found using the TK-4 central-bank compose template. It is the first
// step of the found sequence and owns the full Besu lifecycle so mode:found
// needs no externally pre-started node:
//
//  1. it stages the data-dir scaffold the compose's genesis-init service expects
//     (config/qbftConfigFile.json with the manifest chainId, and the
//     nodes/central-bank/data directory),
//  2. it drives `docker compose up -d` directly (no internal shell script,
//     mirroring startPaladinStep), and
//  3. it polls eth_blockNumber until the node is producing blocks.
//
// genesis-init generates genesis.json only on the first run and is a no-op when
// it already exists, so this step never regenerates genesis on a running spoke.
type startBesuFoundStep struct {
	spokeID        string
	chainID        int
	dataDir        string
	composePath    string
	besuRPCURL     string
	advertisedHost string
	besuImage      string
	rpcPort        int
	wsPort         int
	p2pPort        int
	hostUID        int
	hostGID        int
	healthTimeout  time.Duration
	healthInterval time.Duration
	httpClient     *http.Client
}

func newStartBesuFoundStep(spokeID string, chainID int, dataDir, composePath, besuRPCURL, advertisedHost, besuImage string, rpcPort, wsPort, p2pPort int, healthTimeout, healthInterval time.Duration) Step {
	return &startBesuFoundStep{
		spokeID:        spokeID,
		chainID:        chainID,
		dataDir:        dataDir,
		composePath:    composePath,
		besuRPCURL:     besuRPCURL,
		advertisedHost: advertisedHost,
		besuImage:      besuImage,
		rpcPort:        rpcPort,
		wsPort:         wsPort,
		p2pPort:        p2pPort,
		hostUID:        os.Getuid(),
		hostGID:        os.Getgid(),
		healthTimeout:  healthTimeout,
		healthInterval: healthInterval,
		httpClient:     sharedJoinHTTPClient,
	}
}

func (s *startBesuFoundStep) Name() string { return StepStartBesu }

// Check returns true if the bootnode already responds to eth_blockNumber.
func (s *startBesuFoundStep) Check(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := ethBlockNumber(ctx, s.httpClient, s.besuRPCURL); err != nil {
		return false, nil
	}
	return true, nil
}

func (s *startBesuFoundStep) Run(ctx context.Context) error {
	if err := s.scaffold(); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", s.composePath, "up", "-d")
	cmd.Env = s.composeEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compose up: %w\noutput:\n%s", err, out)
	}

	// Poll until the node answers eth_blockNumber (genesis-init has run and Besu
	// is producing blocks).
	deadline := time.Now().Add(s.healthTimeout)
	for time.Now().Before(deadline) {
		cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		_, rpcErr := ethBlockNumber(cctx, s.httpClient, s.besuRPCURL)
		cancel()
		if rpcErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.healthInterval):
		}
	}
	return fmt.Errorf("besu health check timed out after %s", s.healthTimeout)
}

// scaffold creates the data-dir layout the genesis-init service bind-mounts and
// renders config/qbftConfigFile.json (with the spoke chainId) if absent. It is
// non-destructive: an existing qbftConfigFile.json is never overwritten.
func (s *startBesuFoundStep) scaffold() error {
	for _, sub := range []string{
		filepath.Join(s.dataDir, "config"),
		filepath.Join(s.dataDir, "genesis"),
		filepath.Join(s.dataDir, "nodes", "central-bank", "data"),
	} {
		if err := os.MkdirAll(sub, 0o755); err != nil {
			return fmt.Errorf("scaffold mkdir %s: %w", sub, err)
		}
	}

	qbftPath := filepath.Join(s.dataDir, "config", "qbftConfigFile.json")
	if _, err := os.Stat(qbftPath); err == nil {
		return nil // preserve operator-provided config
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("scaffold stat %s: %w", qbftPath, err)
	}

	data, err := s.renderQBFTConfig()
	if err != nil {
		return fmt.Errorf("render qbft config: %w", err)
	}
	if err := os.WriteFile(qbftPath, data, 0o644); err != nil {
		return fmt.Errorf("write qbft config: %w", err)
	}
	return nil
}

// renderQBFTConfig builds the QBFT blockchain config used by
// `besu operator generate-blockchain-config`, with the spoke's chainId injected.
// Mirrors provisioning/templates/central-bank/examples/qbftConfigFile.json.
func (s *startBesuFoundStep) renderQBFTConfig() ([]byte, error) {
	cfg := map[string]any{
		"genesis": map[string]any{
			"config": map[string]any{
				"chainId":     s.chainID,
				"londonBlock": 0,
				"qbft": map[string]any{
					"blockperiodseconds":    2,
					"epochlength":           30000,
					"requesttimeoutseconds": 4,
				},
			},
			"nonce":      "0x0",
			"timestamp":  "0x0",
			"gasLimit":   "0x1fffffffffffff",
			"difficulty": "0x1",
			"mixHash":    "0x0000000000000000000000000000000000000000000000000000000000000000",
			"coinbase":   "0x0000000000000000000000000000000000000000",
			"alloc":      map[string]any{},
		},
		"blockchain": map[string]any{
			"nodes": map[string]any{
				"generate": true,
				"count":    1,
			},
		},
	}
	return json.MarshalIndent(cfg, "", "  ")
}

// composeEnv builds the environment for the central-bank compose template.
// It deliberately omits BOOTNODE_ENODE: the central bank IS the bootnode.
func (s *startBesuFoundStep) composeEnv() []string {
	return append(os.Environ(),
		"SPOKE_ID="+s.spokeID,
		"SPOKE_DATA_DIR="+s.dataDir,
		"BESU_IMAGE="+s.besuImage,
		"BESU_ADVERTISED_HOST="+s.advertisedHost,
		"BESU_RPC_PORT="+strconv.Itoa(s.rpcPort),
		"BESU_WS_PORT="+strconv.Itoa(s.wsPort),
		"BESU_P2P_PORT="+strconv.Itoa(s.p2pPort),
		"HOST_UID="+strconv.Itoa(s.hostUID),
		"HOST_GID="+strconv.Itoa(s.hostGID),
	)
}

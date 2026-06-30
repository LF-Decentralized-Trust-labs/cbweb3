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

// defaultPaladinImage is the pinned Paladin image used for the spoke's Paladin
// nodes when the deps do not override it. Matches the reference network.
const defaultPaladinImage = "docker.io/lfdecentralizedtrust/paladin:v0.15.0-rc.1"

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

	// Unique compose project per entity: the central-bank compose template is shared
	// across spokes, so without -p every CB shares one project and a second spoke's
	// found would reconcile (and remove) the first spoke's Besu node.
	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", s.spokeID+"-cb-besu", "-f", s.composePath, "up", "-d")
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

// devGenesisAllocAddresses are the standard Hyperledger Besu dev accounts that
// the reference network pre-funds in genesis (see
// deploy/local/spoke-besu-a/config/qbftConfigFile.json). The deploy-contracts
// scripts deploy from 0xFE3B557E…, so it must be funded; the rest match the
// reference set so script behaviour is identical across spokes.
//
// Only balances are written here — never the dev private keys. Those keys live
// in the reference scripts that already hold them; participant identity keys go
// through the KeyProvider, never into genesis.
var devGenesisAllocAddresses = []string{
	"fe3b557e8fb62b89f4916b721be55ceb828dbd73",
	"c5fdf4076b8f3a5357c5e395ab970b5b54098fef",
	"c110089385bad5026e5083443c3b443806da42df",
	"627306090abab3a6e1400e9345bc60c78a8bef57",
	"f17f52151ebef6c7334fad080c5704d77216b732",
	"a2ef7fad3fb7705424b7cc27d21526828dc08ae8",
	"17e2fb46c3c8445cf458fe94b795e32ae50faba9",
	"3fcb45e5fed0c339e621bf1c2f87a3d1fb92e399",
	"ccba841b4ca824e72609609f79dee43a99e44b74",
	"12e28f145001ce04ccb26cbc944c244613d47270",
	"7834b5acfd91706b4d11046ac7f296ca2e9528ec",
	"2d5bec6764271cdde02e6a7273ae1e65d58156af",
}

// devGenesisBalance is 10,000 ETH in wei — the reference per-account funding.
const devGenesisBalance = "10000000000000000000000"

// renderQBFTConfig builds the QBFT blockchain config used by
// `besu operator generate-blockchain-config`, with the spoke's chainId injected.
// Mirrors provisioning/templates/central-bank/examples/qbftConfigFile.json,
// pre-funding the standard dev accounts so contract deployment can proceed.
func (s *startBesuFoundStep) renderQBFTConfig() ([]byte, error) {
	alloc := make(map[string]any, len(devGenesisAllocAddresses))
	for _, addr := range devGenesisAllocAddresses {
		alloc[addr] = map[string]any{"balance": devGenesisBalance}
	}

	cfg := map[string]any{
		"genesis": map[string]any{
			// All fork blocks at 0 plus shanghaiTime/cancunTime so modern Solidity
			// bytecode (PUSH0 etc.) deploys without reverting. zeroBaseFee lets
			// deploys send gasPrice 0. Mirrors the reference network genesis.config.
			"config": map[string]any{
				"chainId":             s.chainID,
				"homesteadBlock":      0,
				"eip150Block":         0,
				"eip155Block":         0,
				"eip158Block":         0,
				"byzantiumBlock":      0,
				"constantinopleBlock": 0,
				"petersburgBlock":     0,
				"istanbulBlock":       0,
				"berlinBlock":         0,
				"londonBlock":         0,
				"preMergeForkBlock":   0,
				"shanghaiTime":        0,
				"cancunTime":          0,
				"qbft": map[string]any{
					"blockperiodseconds":    2,
					"epochlength":           30000,
					"requesttimeoutseconds": 4,
				},
				"zeroBaseFee": true,
			},
			"nonce":      "0x0",
			"timestamp":  "0x58ee40ba",
			"gasLimit":   "0x1c9c380",
			"difficulty": "0x1",
			// QBFT magic mixHash ("critical byzantine fault tolerance").
			"mixHash":  "0x63746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365",
			"coinbase": "0x0000000000000000000000000000000000000000",
			"alloc":    alloc,
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

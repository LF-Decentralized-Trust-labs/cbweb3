// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// startBesuJoinStep brings up the commercial-bank Besu node (TK-8 template)
// as a non-validator that syncs from the spoke bootnode via BOOTNODE_ENODE.
type startBesuJoinStep struct {
	spokeID        string
	bankID         string
	dataDir        string
	composePath    string
	besuRPCURL     string
	bootnodeEnode  string
	advertisedHost string
	rpcPort        int
	wsPort         int
	p2pPort        int
	besuImage      string
	httpClient     *http.Client
}

func newStartBesuJoinStep(spokeID, bankID, dataDir, composePath, besuRPCURL, bootnodeEnode, advertisedHost, besuImage string, rpcPort, wsPort, p2pPort int) Step {
	return &startBesuJoinStep{
		spokeID:        spokeID,
		bankID:         bankID,
		dataDir:        dataDir,
		composePath:    composePath,
		besuRPCURL:     besuRPCURL,
		bootnodeEnode:  bootnodeEnode,
		advertisedHost: advertisedHost,
		rpcPort:        rpcPort,
		wsPort:         wsPort,
		p2pPort:        p2pPort,
		besuImage:      besuImage,
		httpClient:     sharedJoinHTTPClient,
	}
}

func (s *startBesuJoinStep) Name() string { return StepStartBesuJoin }

// Check returns true if the node already responds to eth_blockNumber.
func (s *startBesuJoinStep) Check(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := ethBlockNumber(ctx, s.httpClient, s.besuRPCURL); err != nil {
		return false, nil
	}
	return true, nil
}

func (s *startBesuJoinStep) Run(ctx context.Context) error {
	if s.bootnodeEnode == "" {
		return fmt.Errorf("start-besu-join: BOOTNODE_ENODE is required for mode:join")
	}
	// Unique compose project per entity: the commercial-bank compose template is
	// shared across banks, so without -p every bank shares one project and a later
	// bank's join would reconcile (and remove) an earlier bank's Besu node.
	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", s.spokeID+"-"+s.bankID+"-besu", "-f", s.composePath, "up", "-d")
	cmd.Env = s.composeEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compose up: %w\noutput:\n%s", err, out)
	}
	return nil
}

func (s *startBesuJoinStep) composeEnv() []string {
	return append(os.Environ(),
		"SPOKE_ID="+s.spokeID,
		"BANK_ID="+s.bankID,
		"SPOKE_DATA_DIR="+s.dataDir,
		"BOOTNODE_ENODE="+s.bootnodeEnode,
		"BESU_ADVERTISED_HOST="+s.advertisedHost,
		"BESU_IMAGE="+s.besuImage,
		"BESU_RPC_PORT="+strconv.Itoa(s.rpcPort),
		"BESU_WS_PORT="+strconv.Itoa(s.wsPort),
		"BESU_P2P_PORT="+strconv.Itoa(s.p2pPort),
	)
}

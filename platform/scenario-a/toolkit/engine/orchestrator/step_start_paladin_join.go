// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// startPaladinJoinStep brings up the commercial bank's Paladin node (mode:join)
// using the commercial-bank Paladin compose template, then polls until it is
// healthy. Mirrors startPaladinStep but for a single bank node attaching to the
// spoke's external Besu network.
type startPaladinJoinStep struct {
	spokeID        string
	bankID         string
	dataDir        string
	composePath    string
	paladinImage   string
	paladinRPCURL  string
	besuRPCPort    int
	healthTimeout  time.Duration
	healthInterval time.Duration
}

func newStartPaladinJoinStep(spokeID, bankID, dataDir, composePath, paladinImage string, besuRPCPort int, healthTimeout, healthInterval time.Duration) Step {
	return &startPaladinJoinStep{
		spokeID:        spokeID,
		bankID:         bankID,
		dataDir:        dataDir,
		composePath:    composePath,
		paladinImage:   paladinImage,
		paladinRPCURL:  fmt.Sprintf("http://localhost:%d", besuRPCPort+bankPaladinRPCPortOffset),
		besuRPCPort:    besuRPCPort,
		healthTimeout:  healthTimeout,
		healthInterval: healthInterval,
	}
}

// Bank Paladin host ports are derived from the bank's (unique) Besu RPC port, each
// in its OWN +1000 band — NOT 3 consecutive ports. Consecutive ports made adjacent
// banks' (and the CB's) 3-wide bands overlap (e.g. besu 8646 → 31646-31648 hit the
// CB's fixed 31648). Separate bands mean adjacent Besu ports differ by 1 per band
// and never collide, and the bank range (27xxx-29xxx) clears the CB's (31648-31650).
const (
	bankPaladinRPCPortOffset  = 19000
	bankPaladinWSPortOffset   = 20000
	bankPaladinGRPCPortOffset = 21000
)

func (s *startPaladinJoinStep) Name() string { return StepStartPaladinJoin }

func (s *startPaladinJoinStep) Check(ctx context.Context) (bool, error) {
	return paladinHealthyAt(ctx, s.paladinRPCURL), nil
}

func (s *startPaladinJoinStep) Run(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", s.composePath, "up", "-d")
	cmd.Env = s.composeEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("compose up: %w\noutput:\n%s", err, out)
	}
	deadline := time.Now().Add(s.healthTimeout)
	for time.Now().Before(deadline) {
		if paladinHealthyAt(ctx, s.paladinRPCURL) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.healthInterval):
		}
	}
	return fmt.Errorf("bank paladin health check timed out after %s", s.healthTimeout)
}

func (s *startPaladinJoinStep) composeEnv() []string {
	image := s.paladinImage
	if image == "" {
		image = defaultPaladinImage
	}
	return append(os.Environ(),
		"SPOKE_ID="+s.spokeID,
		"BANK_ID="+s.bankID,
		"SPOKE_DATA_DIR="+s.dataDir,
		"PALADIN_IMAGE="+image,
		"PALADIN_BANK_RPC_PORT="+strconv.Itoa(s.besuRPCPort+bankPaladinRPCPortOffset),
		"PALADIN_BANK_WS_PORT="+strconv.Itoa(s.besuRPCPort+bankPaladinWSPortOffset),
		"PALADIN_BANK_GRPC_PORT="+strconv.Itoa(s.besuRPCPort+bankPaladinGRPCPortOffset),
		"SPOKE_NETWORK_NAME=cbweb3-"+s.spokeID+"-besu",
		"PALADIN_UID="+strconv.Itoa(os.Getuid()),
		"PALADIN_GID="+strconv.Itoa(os.Getgid()),
	)
}

// paladinHealthyAt returns true if the Paladin RPC at url answers with the
// PD020704 (unknown transaction) error, signalling it is up. Standalone variant
// of startPaladinStep.paladinHealthy for the join node.
func paladinHealthyAt(ctx context.Context, url string) bool {
	body := `{"jsonrpc":"2.0","id":1,"method":"ptx_getTransaction","params":["dummy"]}`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBufferString(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "PD020704")
}

// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type startPaladinStep struct {
	spokeID        string
	dataDir        string
	composePath    string
	paladinCBURL   string
	paladinImage   string
	healthTimeout  time.Duration
	healthInterval time.Duration
}

func newStartPaladinStep(spokeID, dataDir, composePath, paladinCBURL, paladinImage string, healthTimeout, healthInterval time.Duration) Step {
	return &startPaladinStep{
		spokeID:        spokeID,
		dataDir:        dataDir,
		composePath:    composePath,
		paladinCBURL:   paladinCBURL,
		paladinImage:   paladinImage,
		healthTimeout:  healthTimeout,
		healthInterval: healthInterval,
	}
}

func (s *startPaladinStep) Name() string { return StepStartPaladin }

// Check verifies: container running AND ptx_getTransaction returns PD020704.
func (s *startPaladinStep) Check(ctx context.Context) (bool, error) {
	if !s.containerRunning(ctx) {
		return false, nil
	}
	return s.paladinHealthy(ctx), nil
}

func (s *startPaladinStep) Run(ctx context.Context) error {
	// Stop → clean volumes → start.
	if err := s.composeDown(ctx); err != nil {
		return fmt.Errorf("compose down: %w", err)
	}
	if err := s.removeVolumes(ctx); err != nil {
		return fmt.Errorf("remove volumes: %w", err)
	}
	if err := s.composeUp(ctx); err != nil {
		return fmt.Errorf("compose up: %w", err)
	}
	// Poll until healthy.
	deadline := time.Now().Add(s.healthTimeout)
	for time.Now().Before(deadline) {
		if s.paladinHealthy(ctx) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.healthInterval):
		}
	}
	return fmt.Errorf("paladin health check timed out after %s", s.healthTimeout)
}

func (s *startPaladinStep) composeEnv() []string {
	image := s.paladinImage
	if image == "" {
		image = defaultPaladinImage
	}
	// Derive Paladin CB host ports from the RPC URL. WS = RPC+1, gRPC = RPC+2 —
	// deterministic and unique per spoke (the RPC port is per-spoke). Internal
	// container ports (8548/8549/9000) are fixed by the template.
	rpcPort := paladinHostPort(s.paladinCBURL, 31648)
	return append(os.Environ(),
		"SPOKE_ID="+s.spokeID,
		"SPOKE_DATA_DIR="+s.dataDir,
		"PALADIN_IMAGE="+image,
		"PALADIN_CB_RPC_PORT="+strconv.Itoa(rpcPort),
		"PALADIN_CB_WS_PORT="+strconv.Itoa(rpcPort+1),
		"PALADIN_CB_GRPC_PORT="+strconv.Itoa(rpcPort+2),
		// The Paladin nodes attach to the external Besu network created by the
		// central-bank Besu compose (default name cbweb3-<spoke>-besu).
		"SPOKE_NETWORK_NAME=cbweb3-"+s.spokeID+"-besu",
		"PALADIN_UID="+strconv.Itoa(os.Getuid()),
		"PALADIN_GID="+strconv.Itoa(os.Getgid()),
	)
}

// paladinHostPort extracts the TCP port from a URL like "http://localhost:31648".
// Returns fallback when the URL has no parseable port.
func paladinHostPort(rawURL string, fallback int) int {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fallback
	}
	if p := u.Port(); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			return n
		}
	}
	return fallback
}

// cbPaladinProject is the unique compose project for this spoke's CB Paladin. The
// central-bank Paladin template is shared across spokes, so without -p every CB
// Paladin shares one project and a second spoke's found would reconcile (and
// remove) the first spoke's Paladin node. Used consistently by up/down/ps.
func (s *startPaladinStep) cbPaladinProject() string { return s.spokeID + "-cb-paladin" }

func (s *startPaladinStep) composeDown(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", s.cbPaladinProject(), "-f", s.composePath, "down")
	cmd.Env = s.composeEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\noutput:\n%s", err, out)
	}
	return nil
}

func (s *startPaladinStep) removeVolumes(ctx context.Context) error {
	// Remove Paladin data volumes for this spoke (block-indexer must re-sync from block 0).
	volumes := []string{
		s.spokeID + "_paladin_cb_data",
		s.spokeID + "_paladin_bank_a_data",
		s.spokeID + "_paladin_bank_c_data",
	}
	args := append([]string{"volume", "rm", "-f"}, volumes...)
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "No such volume") {
		return fmt.Errorf("%w\noutput:\n%s", err, out)
	}
	return nil
}

func (s *startPaladinStep) composeUp(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", s.cbPaladinProject(), "-f", s.composePath, "up", "-d")
	cmd.Env = s.composeEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\noutput:\n%s", err, out)
	}
	return nil
}

func (s *startPaladinStep) containerRunning(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", s.cbPaladinProject(), "-f", s.composePath, "ps", "--format", "json")
	cmd.Env = s.composeEnv()
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	containerName := "paladin-" + s.spokeID + "-cb"
	return strings.Contains(string(out), containerName) && strings.Contains(string(out), "running")
}

func (s *startPaladinStep) paladinHealthy(ctx context.Context) bool {
	body := `{"jsonrpc":"2.0","id":1,"method":"ptx_getTransaction","params":["dummy"]}`
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.paladinCBURL,
		bytes.NewBufferString(body))
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
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return false
	}
	// Paladin returns error code PD020704 for unknown transaction — this means it's up.
	if errObj, ok := result["error"].(map[string]any); ok {
		if code, ok := errObj["code"]; ok {
			return strings.Contains(fmt.Sprintf("%v", code), "PD020704") ||
				strings.Contains(fmt.Sprintf("%v", result), "PD020704")
		}
	}
	return strings.Contains(string(data), "PD020704")
}

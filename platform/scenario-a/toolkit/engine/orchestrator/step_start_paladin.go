// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type startPaladinStep struct {
	spokeID           string
	dataDir           string
	composePath       string
	paladinCBURL      string
	healthTimeout     time.Duration
	healthInterval    time.Duration
}

func newStartPaladinStep(spokeID, dataDir, composePath, paladinCBURL string, healthTimeout, healthInterval time.Duration) Step {
	return &startPaladinStep{
		spokeID:        spokeID,
		dataDir:        dataDir,
		composePath:    composePath,
		paladinCBURL:   paladinCBURL,
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
	return append(os.Environ(),
		"SPOKE_ID="+s.spokeID,
		"SPOKE_DATA_DIR="+s.dataDir,
	)
}

func (s *startPaladinStep) composeDown(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", s.composePath, "down")
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
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", s.composePath, "up", "-d")
	cmd.Env = s.composeEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\noutput:\n%s", err, out)
	}
	return nil
}

func (s *startPaladinStep) containerRunning(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", s.composePath, "ps", "--format", "json")
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

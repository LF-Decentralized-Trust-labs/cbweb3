// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type registerNodesStep struct {
	spokeID    string
	dataDir    string
	besuRPCURL string
	scriptsDir string
	timeout    time.Duration
}

func newRegisterNodesStep(spokeID, dataDir, besuRPCURL, scriptsDir string, timeout time.Duration) Step {
	return &registerNodesStep{spokeID: spokeID, dataDir: dataDir, besuRPCURL: besuRPCURL, scriptsDir: scriptsDir, timeout: timeout}
}

func (s *registerNodesStep) Name() string { return StepRegisterNodes }

// Check uses only the provisioning state file (no idempotent external query available for Paladin node registry).
func (s *registerNodesStep) Check(_ context.Context) (bool, error) {
	state, err := loadState(s.dataDir)
	if err != nil {
		return false, err
	}
	return statusFor(state, StepRegisterNodes) == "done", nil
}

func (s *registerNodesStep) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "test", "./...",
		"-run", "TestRegisterPaladinNodes", "-v", "-count=1",
		fmt.Sprintf("-timeout=%s", s.timeout))
	cmd.Dir = s.scriptsDir
	cmd.Env = append(os.Environ(), "SPOKE="+s.spokeID, "BESU_RPC_URL="+s.besuRPCURL)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		output := out.String()
		// "already registered" is treated as idempotent success.
		if strings.Contains(output, "already registered") {
			return nil
		}
		return fmt.Errorf("go test TestRegisterPaladinNodes: %w\noutput:\n%s", err, output)
	}
	return nil
}

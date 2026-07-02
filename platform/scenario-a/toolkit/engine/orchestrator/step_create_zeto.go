// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type createZetoStep struct {
	spokeID      string
	dataDir      string
	paladinCBURL string
	scriptsDir   string
	timeout      time.Duration
}

func newCreateZetoStep(spokeID, dataDir, paladinCBURL, scriptsDir string, timeout time.Duration) Step {
	return &createZetoStep{spokeID: spokeID, dataDir: dataDir, paladinCBURL: paladinCBURL, scriptsDir: scriptsDir, timeout: timeout}
}

func (s *createZetoStep) Name() string { return StepCreateZetoToken }

func (s *createZetoStep) Check(_ context.Context) (bool, error) {
	addrs, err := parseDeployedAddrs(filepath.Join(s.dataDir, ".deployed-addrs.env"))
	if err != nil {
		return false, err
	}
	return addrs.ZetoTokenAddress != "", nil
}

func (s *createZetoStep) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "./...",
		"-run", "TestCreateZetoTokenInstance", "-v", "-count=1",
		fmt.Sprintf("-timeout=%s", s.timeout))
	cmd.Dir = s.scriptsDir
	cmd.Env = append(os.Environ(), "SPOKE="+s.spokeID, "PALADIN_CB_URL="+s.paladinCBURL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("TestCreateZetoTokenInstance: %w\noutput:\n%s", err, out)
	}

	// The script appends ZETO_TOKEN_ADDRESS to <scriptsDir>/../<spoke>/.deployed-addrs.env;
	// sync it into dataDir so Check (idempotency) and the bundle emitter see it.
	src := filepath.Join(s.scriptsDir, "..", s.spokeID, ".deployed-addrs.env")
	dst := filepath.Join(s.dataDir, ".deployed-addrs.env")
	if src != dst {
		data, rerr := os.ReadFile(src)
		if rerr != nil {
			return fmt.Errorf("read deployed-addrs after zeto: %w", rerr)
		}
		if werr := os.WriteFile(dst, data, 0o644); werr != nil {
			return fmt.Errorf("sync deployed-addrs to dataDir: %w", werr)
		}
	}
	return nil
}

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

type deployContractsStep struct {
	spokeID    string
	dataDir    string
	besuRPCURL string
	scriptsDir string
	timeout    time.Duration
}

func newDeployContractsStep(spokeID, dataDir, besuRPCURL, scriptsDir string, timeout time.Duration) Step {
	return &deployContractsStep{spokeID: spokeID, dataDir: dataDir, besuRPCURL: besuRPCURL, scriptsDir: scriptsDir, timeout: timeout}
}

func (s *deployContractsStep) Name() string { return StepDeployContracts }

func (s *deployContractsStep) Check(_ context.Context) (bool, error) {
	addrs, err := parseDeployedAddrs(filepath.Join(s.dataDir, ".deployed-addrs.env"))
	if err != nil {
		return false, err
	}
	return addrs.RegistryContractAddress != "" &&
		addrs.ZetoFactoryAddress != "" &&
		addrs.PenteFactoryAddress != "", nil
}

func (s *deployContractsStep) Run(ctx context.Context) error {
	// The reference deploy scripts write .deployed-addrs.env to
	// <scriptsDir>/../<spokeID>/ (addrsEnvFile = "../$SPOKE/.deployed-addrs.env").
	// That per-spoke directory does not exist for an arbitrary new spoke id, so
	// create it before invoking the scripts (the scripts must not be modified).
	addrDir := filepath.Join(s.scriptsDir, "..", s.spokeID)
	if err := os.MkdirAll(addrDir, 0o755); err != nil {
		return fmt.Errorf("create deployed-addrs dir %q: %w", addrDir, err)
	}

	for _, testName := range []string{"TestDeployEVMRegistry", "TestDeployZetoFactory", "TestDeployPenteFactory"} {
		if err := s.runGoTest(ctx, testName); err != nil {
			return fmt.Errorf("%s: %w", testName, err)
		}
	}
	// Scripts write .deployed-addrs.env relative to scriptsDir; copy to dataDir.
	src := filepath.Join(s.scriptsDir, "..", s.spokeID, ".deployed-addrs.env")
	dst := filepath.Join(s.dataDir, ".deployed-addrs.env")
	if src != dst {
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read deployed-addrs: %w", err)
		}
		if err := os.MkdirAll(s.dataDir, 0o755); err != nil {
			return fmt.Errorf("mkdir dataDir: %w", err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return fmt.Errorf("write deployed-addrs: %w", err)
		}
	}
	return nil
}

func (s *deployContractsStep) runGoTest(ctx context.Context, testName string) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "./...", "-run", testName, "-v", "-count=1",
		fmt.Sprintf("-timeout=%s", s.timeout))
	cmd.Dir = s.scriptsDir
	cmd.Env = append(os.Environ(), "SPOKE="+s.spokeID, "BESU_RPC_URL="+s.besuRPCURL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go test %s: %w\noutput:\n%s", testName, err, out)
	}
	return nil
}

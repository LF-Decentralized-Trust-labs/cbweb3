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

type deployFXAStep struct {
	spokeID      string
	dataDir      string
	paladinCBURL string
	scriptsDir   string
	timeout      time.Duration
}

func newDeployFXAStep(spokeID, dataDir, paladinCBURL, scriptsDir string, timeout time.Duration) Step {
	return &deployFXAStep{spokeID: spokeID, dataDir: dataDir, paladinCBURL: paladinCBURL, scriptsDir: scriptsDir, timeout: timeout}
}

func (s *deployFXAStep) Name() string { return StepDeployFXAPente }

func (s *deployFXAStep) Check(_ context.Context) (bool, error) {
	addrs, err := parseDeployedAddrs(filepath.Join(s.dataDir, ".deployed-addrs.env"))
	if err != nil {
		return false, err
	}
	return addrs.FXAgreementDeployedAt != "", nil
}

func (s *deployFXAStep) Run(ctx context.Context) error {
	addrs, err := parseDeployedAddrs(filepath.Join(s.dataDir, ".deployed-addrs.env"))
	if err != nil {
		return fmt.Errorf("read deployed-addrs: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "./...",
		"-run", "TestDeployFXAgreementPente", "-v", "-count=1",
		fmt.Sprintf("-timeout=%s", s.timeout))
	cmd.Dir = s.scriptsDir
	cmd.Env = append(os.Environ(),
		"SPOKE="+s.spokeID,
		"PALADIN_CB_URL="+s.paladinCBURL,
		"REGISTRY_CONTRACT_ADDRESS="+addrs.RegistryContractAddress,
		"PENTE_CONTEXT_GROUP_ID="+addrs.PenteContextGroupID,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("TestDeployFXAgreementPente: %w\noutput:\n%s", err, out)
	}
	return nil
}

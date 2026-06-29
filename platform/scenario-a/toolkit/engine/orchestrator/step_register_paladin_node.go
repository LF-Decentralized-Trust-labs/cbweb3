// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// registerPaladinNodeStep registers the joining commercial bank's Paladin node
// identity on-chain (mode:join), using the same native primitive as the found CB
// node — parametrized by spoke id and bank id, no hardcoded bank list. Mirrors
// spk-02 register-paladin-nodes for the joiner.
type registerPaladinNodeStep struct {
	spokeID      string
	bankID       string
	dataDir      string
	besuRPCURL   string
	registryAddr string
	timeout      time.Duration
}

func newRegisterPaladinNodeStep(spokeID, bankID, dataDir, besuRPCURL, registryAddr string, timeout time.Duration) Step {
	return &registerPaladinNodeStep{
		spokeID:      spokeID,
		bankID:       bankID,
		dataDir:      dataDir,
		besuRPCURL:   besuRPCURL,
		registryAddr: registryAddr,
		timeout:      timeout,
	}
}

func (s *registerPaladinNodeStep) Name() string { return StepRegisterPaladinNode }

// Check uses the provisioning state file (no idempotent on-chain query exposed).
func (s *registerPaladinNodeStep) Check(_ context.Context) (bool, error) {
	state, err := LoadState(s.dataDir)
	if err != nil {
		return false, err
	}
	return statusFor(state, StepRegisterPaladinNode) == "done", nil
}

func (s *registerPaladinNodeStep) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	if s.registryAddr == "" {
		return fmt.Errorf("bundle registry address is empty")
	}
	certPath := filepath.Join(s.dataDir, "paladin", s.bankID, "tls.crt")
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("read bank Paladin cert %s: %w", certPath, err)
	}

	deployerKey, err := devKey(registryDeployerKey)
	if err != nil {
		return fmt.Errorf("decode registry deployer key: %w", err)
	}
	ownerKey, err := devKey(bankNodeOwnerKey)
	if err != nil {
		return fmt.Errorf("decode bank node owner key: %w", err)
	}

	return registerPaladinNode(ctx, s.besuRPCURL, paladinNodeRegistration{
		registry:     common.HexToAddress(s.registryAddr),
		nodeName:     bankNodeName(s.spokeID, s.bankID),
		grpcHostname: bankGrpcHostname(s.spokeID, s.bankID),
		certPEM:      certPEM,
		deployerKey:  deployerKey,
		ownerKey:     ownerKey,
	})
}

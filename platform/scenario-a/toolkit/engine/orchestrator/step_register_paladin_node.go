// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"

	kp "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
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
	keyProvider  kp.KeyProvider
	timeout      time.Duration
}

func newRegisterPaladinNodeStep(spokeID, bankID, dataDir, besuRPCURL, registryAddr string, keyProvider kp.KeyProvider, timeout time.Duration) Step {
	return &registerPaladinNodeStep{
		spokeID:      spokeID,
		bankID:       bankID,
		dataDir:      dataDir,
		besuRPCURL:   besuRPCURL,
		registryAddr: registryAddr,
		keyProvider:  keyProvider,
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
	// Written by gen-tls-join into the named volume — no host filesystem
	// involved (deviation from the original SPOKE_DATA_DIR bind-mount design;
	// see specs/032-commercial-bank-join/research.md addendum).
	paladinConfigVolume := s.spokeID + "_" + s.bankID + "_paladin_config"
	certPEM, err := readVolumeFile(ctx, paladinConfigVolume, "tls.crt")
	if err != nil {
		return fmt.Errorf("read bank Paladin cert from volume %s: %w", paladinConfigVolume, err)
	}

	return registerPaladinNode(ctx, s.besuRPCURL, paladinNodeRegistration{
		registry:     common.HexToAddress(s.registryAddr),
		nodeName:     bankNodeName(s.spokeID, s.bankID),
		grpcHostname: bankGrpcHostname(s.spokeID, s.bankID),
		certPEM:      certPEM,
		provider:     s.keyProvider,
		signerKeyID:  kp.LocalOperatorKeyID,
	})
}

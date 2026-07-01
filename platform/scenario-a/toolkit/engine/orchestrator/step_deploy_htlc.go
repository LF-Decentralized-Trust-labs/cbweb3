// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/ethereum/go-ethereum/common"

	kp "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
)

// deployHTLCStep deploys HashTimeLockedContract — Scenario A's domestic settlement
// leg (lock + secret reveal) — to the spoke chain and persists HTLC_ADDRESS. Its
// constructor takes the spoke IdentityRegistry (PARTICIPANT_REGISTRY_ADDRESS, from
// onboard-registry); the FXAgreement and CommitmentHashRegistry slots are the zero
// address (FX negotiation is a service-layer/Paladin concern, matching the legacy
// DeployCBWeb3Spoke script). Signs via the KeyProvider.
type deployHTLCStep struct {
	dataDir      string
	besuRPCURL   string
	keyProvider  kp.KeyProvider
	htlcArtifact string // path to HashTimeLockedContract.sol Foundry artifact
	timeout      time.Duration
}

func newDeployHTLCStep(dataDir, besuRPCURL string, keyProvider kp.KeyProvider, htlcArtifact string, timeout time.Duration) Step {
	return &deployHTLCStep{
		dataDir:      dataDir,
		besuRPCURL:   besuRPCURL,
		keyProvider:  keyProvider,
		htlcArtifact: htlcArtifact,
		timeout:      timeout,
	}
}

func (s *deployHTLCStep) Name() string { return StepDeployHTLC }

func (s *deployHTLCStep) Check(_ context.Context) (bool, error) {
	addrs, err := parseDeployedAddrs(filepath.Join(s.dataDir, ".deployed-addrs.env"))
	if err != nil {
		return false, err
	}
	return addrs.HTLCAddress != "", nil
}

func (s *deployHTLCStep) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	envPath := filepath.Join(s.dataDir, ".deployed-addrs.env")
	addrs, err := parseDeployedAddrs(envPath)
	if err != nil {
		return fmt.Errorf("read deployed-addrs: %w", err)
	}
	if addrs.ParticipantRegistryAddress == "" {
		return fmt.Errorf("PARTICIPANT_REGISTRY_ADDRESS not set — onboard-registry must run before deploy-htlc")
	}

	identityRegistry := common.HexToAddress(addrs.ParticipantRegistryAddress)
	zero := common.Address{}
	deployed, err := deployContractFromArtifact(ctx, s.besuRPCURL, s.htlcArtifact,
		s.keyProvider, kp.LocalOperatorKeyID, identityRegistry, zero, zero)
	if err != nil {
		return fmt.Errorf("deploy HTLC: %w", err)
	}

	if err := addrsAppend(envPath, "HTLC_ADDRESS", deployed.Hex()); err != nil {
		return fmt.Errorf("persist HTLC_ADDRESS: %w", err)
	}
	return nil
}

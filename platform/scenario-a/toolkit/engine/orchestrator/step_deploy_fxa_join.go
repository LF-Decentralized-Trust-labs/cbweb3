// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

// deployFXAJoinStep deploys FXAgreement inside the bilateral Pente group (mode:join
// US3), using the bank's Paladin identity as the sender and the spoke participant
// whitelist (IdentityRegistry.sol) as the FXAgreement constructor's registry.
type deployFXAJoinStep struct {
	spokeID          string
	bankID           string
	dataDir          string
	paladinURL       string
	artifactPath     string // FXAgreement Foundry artifact
	identityRegistry string // participant whitelist address (from join bundle)
	timeout          time.Duration
}

func newDeployFXAJoinStep(spokeID, bankID, dataDir, paladinURL, artifactPath, identityRegistry string, timeout time.Duration) Step {
	return &deployFXAJoinStep{
		spokeID:          spokeID,
		bankID:           bankID,
		dataDir:          dataDir,
		paladinURL:       paladinURL,
		artifactPath:     artifactPath,
		identityRegistry: identityRegistry,
		timeout:          timeout,
	}
}

func (s *deployFXAJoinStep) Name() string { return StepDeployFXAJoin }

func (s *deployFXAJoinStep) Check(_ context.Context) (bool, error) {
	addrs, err := parseDeployedAddrs(filepath.Join(s.dataDir, ".deployed-addrs.env"))
	if err != nil {
		return false, err
	}
	return addrs.FXAgreementDeployedAt != "", nil
}

func (s *deployFXAJoinStep) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	envPath := filepath.Join(s.dataDir, ".deployed-addrs.env")
	addrs, err := parseDeployedAddrs(envPath)
	if err != nil {
		return fmt.Errorf("read deployed-addrs: %w", err)
	}
	if addrs.PenteContextGroupID == "" {
		return fmt.Errorf("PENTE_CONTEXT_GROUP_ID missing — create-pente-context must run first")
	}
	if s.identityRegistry == "" {
		return fmt.Errorf("participant registry address missing from bundle")
	}

	// Wait for the group to be ready to accept transactions before deploying.
	if err := waitPenteGroupReady(ctx, s.paladinURL, addrs.PenteContextGroupID, s.timeout); err != nil {
		return err
	}

	from := paladinIdentity(bankNodeName(s.spokeID, s.bankID))
	addr, err := deployFXAInPente(ctx, s.paladinURL, addrs.PenteContextGroupID, from, s.artifactPath, s.identityRegistry)
	if err != nil {
		return fmt.Errorf("deploy FXAgreement in pente: %w", err)
	}
	if addr != "" {
		if err := addrsAppend(envPath, "FX_AGREEMENT_ADDRESS", addr); err != nil {
			return fmt.Errorf("persist FX_AGREEMENT_ADDRESS: %w", err)
		}
	}
	// Mark completion (used by Check) even if the address resolves later.
	marker := addr
	if marker == "" {
		marker = "deployed"
	}
	if err := addrsAppend(envPath, "FX_AGREEMENT_DEPLOYED_AT", marker); err != nil {
		return fmt.Errorf("persist FX_AGREEMENT_DEPLOYED_AT: %w", err)
	}
	return nil
}

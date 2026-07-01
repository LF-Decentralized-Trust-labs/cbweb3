// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	kp "github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/keyprovider"
)

// deployFiatTokenStep deploys FiatCentralBankMoney (fCeBM) — the public Besu-layer
// ERC-20 fiat asset used in the deposit/escrow/redeem flow — to the spoke chain and
// persists FIAT_TOKEN_ADDRESS. Both roles (DEFAULT_ADMIN_ROLE and CENTRAL_BANK_ROLE)
// are granted to the operator address the backend signs with, so the central bank's
// api-gateway can mint/burn and read balances. Signs via the KeyProvider.
type deployFiatTokenStep struct {
	spokeID      string
	dataDir      string
	besuRPCURL   string
	currency     string
	keyProvider  kp.KeyProvider
	fiatArtifact string // path to FiatCentralBankMoney.sol Foundry artifact
	timeout      time.Duration
}

func newDeployFiatTokenStep(spokeID, dataDir, besuRPCURL, currency string, keyProvider kp.KeyProvider, fiatArtifact string, timeout time.Duration) Step {
	return &deployFiatTokenStep{
		spokeID:      spokeID,
		dataDir:      dataDir,
		besuRPCURL:   besuRPCURL,
		currency:     currency,
		keyProvider:  keyProvider,
		fiatArtifact: fiatArtifact,
		timeout:      timeout,
	}
}

func (s *deployFiatTokenStep) Name() string { return StepDeployFiatToken }

func (s *deployFiatTokenStep) Check(_ context.Context) (bool, error) {
	addrs, err := parseDeployedAddrs(filepath.Join(s.dataDir, ".deployed-addrs.env"))
	if err != nil {
		return false, err
	}
	return addrs.FiatTokenAddress != "", nil
}

func (s *deployFiatTokenStep) Run(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	// The operator (governance) key is both admin and central-bank authority in
	// local: it deploys the token AND is the address the backend's FiatClient signs
	// mint/burn with (and whose balance /token/fiat-balance reads). Prod would grant
	// CENTRAL_BANK_ROLE to a distinct KMS-managed wallet.
	operator, err := keyProviderAddress(ctx, s.keyProvider, kp.LocalOperatorKeyID)
	if err != nil {
		return fmt.Errorf("resolve operator address: %w", err)
	}

	name := "Fiat " + s.currency
	symbol := "fCeBM_" + s.currency
	deployed, err := deployContractFromArtifact(ctx, s.besuRPCURL, s.fiatArtifact,
		s.keyProvider, kp.LocalOperatorKeyID, name, symbol, operator, operator)
	if err != nil {
		return fmt.Errorf("deploy fCeBM: %w", err)
	}

	envPath := filepath.Join(s.dataDir, ".deployed-addrs.env")
	if err := addrsAppend(envPath, "FIAT_TOKEN_ADDRESS", deployed.Hex()); err != nil {
		return fmt.Errorf("persist FIAT_TOKEN_ADDRESS: %w", err)
	}
	return nil
}

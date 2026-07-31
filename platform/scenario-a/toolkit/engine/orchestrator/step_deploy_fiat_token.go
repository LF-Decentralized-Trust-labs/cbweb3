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
	token        FiatTokenMetadata
	keyProvider  kp.KeyProvider
	fiatArtifact string // path to FiatCentralBankMoney.sol Foundry artifact
	timeout      time.Duration
}

// FiatTokenMetadata is the ERC-20 identity of a spoke's fCeBM. Only Currency is
// required: Name and Symbol are optional manifest overrides
// (spec.spoke.fiatTokenName / fiatTokenSymbol) and fall back to the currency-derived
// defaults, so two spokes never share a token identity.
type FiatTokenMetadata struct {
	Currency string
	Name     string
	Symbol   string
}

// name is the ERC-20 name to deploy with ("Fiat BRL" unless overridden).
func (f FiatTokenMetadata) name() string {
	if f.Name != "" {
		return f.Name
	}
	return "Fiat " + f.Currency
}

// symbol is the ERC-20 symbol to deploy with ("fCeBM_BRL" unless overridden). The
// "<prefix>_<ISO>" shape is validated at the manifest layer.
func (f FiatTokenMetadata) symbol() string {
	if f.Symbol != "" {
		return f.Symbol
	}
	return "fCeBM_" + f.Currency
}

func newDeployFiatTokenStep(spokeID, dataDir, besuRPCURL string, token FiatTokenMetadata, keyProvider kp.KeyProvider, fiatArtifact string, timeout time.Duration) Step {
	return &deployFiatTokenStep{
		spokeID:      spokeID,
		dataDir:      dataDir,
		besuRPCURL:   besuRPCURL,
		token:        token,
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

	deployed, err := deployContractFromArtifact(ctx, s.besuRPCURL, s.fiatArtifact,
		s.keyProvider, kp.LocalOperatorKeyID, s.token.name(), s.token.symbol(), operator, operator)
	if err != nil {
		return fmt.Errorf("deploy fCeBM: %w", err)
	}

	envPath := filepath.Join(s.dataDir, ".deployed-addrs.env")
	if err := addrsAppend(envPath, "FIAT_TOKEN_ADDRESS", deployed.Hex()); err != nil {
		return fmt.Errorf("persist FIAT_TOKEN_ADDRESS: %w", err)
	}
	return nil
}

// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/registry/bindings"
)

// BesuConfig holds the configuration required to connect to a Besu node and
// interact with the IdentityRegistry contract.
//
// Environment variables used by services:
//
//	BLOCKCHAIN_CLIENT           — "besu" or "noop" (default: "noop")
//	BESU_RPC_URL                — e.g. "http://besu-node:8545"
//	PARTICIPANT_REGISTRY_ADDRESS — deployed IdentityRegistry address "0x..."
//	CB_PRIVATE_KEY              — Central Bank private key (hex); only required for Central Bank nodes
//	BESU_CHAIN_ID               — Besu network chain ID (default: 1337)
//	BLOCKCHAIN_REQUEST_TIMEOUT_SEC — HTTP timeout in seconds (default: 15)
type BesuConfig struct {
	RPCURL          string
	RegistryAddress string
	ChainID         int64
	RequestTimeout  time.Duration

	// CurrencyRegistryAddress is the deployed CurrencyRegistry contract address
	// ("0x…"), read from CURRENCY_REGISTRY_ADDRESS. Optional: when empty,
	// currency registration is unavailable and RegisterCurrency returns an error;
	// participant registration is unaffected.
	CurrencyRegistryAddress string

	// PairRegistryAddress is the deployed PairRegistry contract address ("0x…"),
	// read from PAIR_REGISTRY_ADDRESS. Optional: when empty, sovereign-pair
	// registration is unavailable and RegisterPair returns an error.
	PairRegistryAddress string
}

// BesuClient implements RegistryWriter and RegistryReader using go-ethereum's
// ethclient and abigen-generated bindings for the IdentityRegistry contract.
//
// Read operations always work — they use eth_call and require no signing key.
//
// Write operations require a TransactionSigner. When signer is nil, writes
// return ErrNoSigner.
type BesuClient struct {
	cfg      BesuConfig
	signer   TransactionSigner
	client   *ethclient.Client
	contract *bindings.IdentityRegistry
}

// NewBesuClient creates a BesuClient connected to the given RPC URL.
// The signer is optional: pass nil for a read-only client where write
// operations will return ErrNoSigner.
func NewBesuClient(cfg BesuConfig, signer TransactionSigner) (*BesuClient, error) {
	if strings.TrimSpace(cfg.RPCURL) == "" {
		return nil, errors.New("registry: BESU_RPC_URL is required")
	}
	if strings.TrimSpace(cfg.RegistryAddress) == "" {
		return nil, errors.New("registry: PARTICIPANT_REGISTRY_ADDRESS is required")
	}

	client, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("registry: connecting to Besu node: %w", err)
	}

	contractAddr := common.HexToAddress(cfg.RegistryAddress)
	contract, err := bindings.NewIdentityRegistry(contractAddr, client)
	if err != nil {
		return nil, fmt.Errorf("registry: binding IdentityRegistry contract: %w", err)
	}

	return &BesuClient{
		cfg:      cfg,
		signer:   signer,
		client:   client,
		contract: contract,
	}, nil
}

// transactOpts builds bind.TransactOpts from the TransactionSigner.
func (b *BesuClient) transactOpts(ctx context.Context) (*bind.TransactOpts, error) {
	if b.signer == nil {
		return nil, ErrNoSigner
	}

	signerAddr, err := b.signer.SignerAddress(ctx)
	if err != nil {
		return nil, fmt.Errorf("registry: resolving signer address: %w", err)
	}

	chainID := big.NewInt(b.cfg.ChainID)
	signer := b.signer

	gasPrice, err := b.client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("registry: fetching gas price: %w", err)
	}

	return &bind.TransactOpts{
		Context:  ctx,
		From:     common.HexToAddress(signerAddr),
		GasPrice: gasPrice,
		Signer: func(_ common.Address, tx *types.Transaction) (*types.Transaction, error) {
			return signer.SignTx(ctx, tx, chainID)
		},
	}, nil
}

// RegisterParticipant registers a new participant on the IdentityRegistry contract.
// Blocks until the transaction is mined or the context is cancelled.
func (b *BesuClient) RegisterParticipant(ctx context.Context, wallet, name, role string, zkPointer [32]byte) (string, error) {
	opts, err := b.transactOpts(ctx)
	if err != nil {
		return "", err
	}

	solidityRole := RoleToSolidityEnum(role)
	account := common.HexToAddress(wallet)

	// Pin an explicit, generous gas limit: eth_estimateGas underestimates this
	// storage-writing call on the local QBFT/zero-gas chain (observed OutOfGas at
	// ~40k), and gas is free on the local genesis, so over-provisioning is safe.
	opts.GasLimit = 500000

	tx, err := b.contract.RegisterParticipant(opts, account, name, solidityRole, zkPointer)
	if err != nil {
		return "", fmt.Errorf("registry: registerParticipant tx: %w", err)
	}
	if err := b.waitMined(ctx, tx); err != nil {
		return tx.Hash().Hex(), fmt.Errorf("registry: registerParticipant wait: %w", err)
	}
	return tx.Hash().Hex(), nil
}

// VerifyParticipant promotes a Pending participant to Verified on the
// IdentityRegistry contract (step 2 of the two-step onboarding). The signer must
// hold VERIFIER_ROLE. Blocks until the transaction is mined or the context is
// cancelled. Reverts (tx status 0) if the participant is not currently Pending.
func (b *BesuClient) VerifyParticipant(ctx context.Context, wallet string) (string, error) {
	opts, err := b.transactOpts(ctx)
	if err != nil {
		return "", err
	}

	account := common.HexToAddress(wallet)
	// Match RegisterParticipant's explicit gas pin: eth_estimateGas underestimates
	// this storage-writing call on the local zero-gas QBFT chain, and gas is free.
	opts.GasLimit = 500000

	tx, err := b.contract.VerifyParticipant(opts, account)
	if err != nil {
		return "", fmt.Errorf("registry: verifyParticipant tx: %w", err)
	}
	if err := b.waitMined(ctx, tx); err != nil {
		return tx.Hash().Hex(), fmt.Errorf("registry: verifyParticipant wait: %w", err)
	}
	return tx.Hash().Hex(), nil
}

// UpdateStatus changes the KYC status of a participant on-chain.
// Blocks until the transaction is mined or the context is cancelled.
func (b *BesuClient) UpdateStatus(ctx context.Context, wallet string, status uint8) (string, error) {
	opts, err := b.transactOpts(ctx)
	if err != nil {
		return "", err
	}

	account := common.HexToAddress(wallet)
	tx, err := b.contract.UpdateStatus(opts, account, status)
	if err != nil {
		return "", fmt.Errorf("registry: updateStatus tx: %w", err)
	}
	if err := b.waitMined(ctx, tx); err != nil {
		return tx.Hash().Hex(), fmt.Errorf("registry: updateStatus wait: %w", err)
	}
	return tx.Hash().Hex(), nil
}

// SetCertFingerprint stores a certificate fingerprint on-chain.
// Blocks until the transaction is mined or the context is cancelled.
func (b *BesuClient) SetCertFingerprint(ctx context.Context, wallet string, fingerprint [32]byte) (string, error) {
	opts, err := b.transactOpts(ctx)
	if err != nil {
		return "", err
	}

	account := common.HexToAddress(wallet)
	tx, err := b.contract.SetCertFingerprint(opts, account, fingerprint)
	if err != nil {
		return "", fmt.Errorf("registry: setCertFingerprint tx: %w", err)
	}
	if err := b.waitMined(ctx, tx); err != nil {
		return tx.Hash().Hex(), fmt.Errorf("registry: setCertFingerprint wait: %w", err)
	}
	return tx.Hash().Hex(), nil
}

// currencyAuthorityPlan is what remains to be done so a sovereign currency's issuance
// authority rests with its own central bank rather than with the hub that registered it.
type currencyAuthorityPlan struct {
	// SetCentralBankOf points IdentityRegistry.getCentralBankOf(token) at the CB. This is on
	// the registry, whose administration legitimately stays with the hub.
	SetCentralBankOf bool
	// GrantCBRole gives the CB CENTRAL_BANK_ROLE on its W-token (mint/burn).
	GrantCBRole bool
	// GrantCBAdmin gives the CB DEFAULT_ADMIN_ROLE on its W-token, so it — and not the hub —
	// decides who may issue its money. Without this the revocation below is cosmetic: the
	// hub could grant CENTRAL_BANK_ROLE back to itself at any time.
	//
	// Only ever planned while the hub STILL administers the token. What this handover protects is
	// narrower than "the CB administers": it is that the HUB does not. Once the hub is out,
	// administration may legitimately rest with a dedicated sovereign identity rather than the CB's
	// gateway (provisioning separates issuance from administration), and this package never sees
	// that address. Planning a grant then would have the hub sign a transaction it no longer has the
	// authority for — it reverts, and the whole re-apply of that spoke fails with it.
	GrantCBAdmin bool
	// RevokeSignerRole takes CENTRAL_BANK_ROLE away from the hub signer, so the hub cannot
	// issue another sovereign's money.
	RevokeSignerRole bool
	// RevokeSignerAdmin takes DEFAULT_ADMIN_ROLE away from the hub signer. Applied LAST: it
	// is what removes the hub's ability to perform any of the steps above.
	RevokeSignerAdmin bool
}

// Empty reports whether the authority already rests where it should.
func (p currencyAuthorityPlan) Empty() bool {
	return !p.SetCentralBankOf && !p.GrantCBRole && !p.GrantCBAdmin &&
		!p.RevokeSignerRole && !p.RevokeSignerAdmin
}

// planCurrencyAuthority decides which handover steps are still outstanding.
//
// Kept pure so the rule is testable without a chain: the on-chain calls around it are thin.
// When the CB *is* the signer (single-entity local stacks), there is nothing to hand over
// and nothing to revoke — revoking would leave the token with no issuer at all.
func planCurrencyAuthority(signer, cb, currentCBOf common.Address, cbHasRole, signerHasRole, cbIsAdmin, signerIsAdmin bool) currencyAuthorityPlan {
	if cb == (common.Address{}) || cb == signer {
		return currencyAuthorityPlan{}
	}
	return currencyAuthorityPlan{
		SetCentralBankOf: currentCBOf != cb,
		GrantCBRole:      !cbHasRole,
		// See GrantCBAdmin: conditioned on the hub still administering, so a token whose
		// administration the sovereign has already moved on is left alone instead of triggering a
		// grant the hub cannot sign.
		GrantCBAdmin:      !cbIsAdmin && signerIsAdmin,
		RevokeSignerRole:  signerHasRole,
		RevokeSignerAdmin: signerIsAdmin,
	}
}

// RegisterCurrency deploys a founding central bank's bridge token (W-token), registers the
// sovereign currency on the CurrencyRegistry, and hands the token's issuance authority to
// that central bank. Signed throughout by this client's signer (the hub governance key,
// holding DEFAULT_ADMIN_ROLE).
//
// The order is forced by the contracts. CurrencyRegistry.registerCurrency requires
// msg.sender == IdentityRegistry.getCentralBankOf(token), and the hub signer is the only
// key available here — a sovereign CB does not hand its key to the hub. So the hub registers
// the currency as the interim central bank and then hands over:
//
//	deploy(centralBank = signer) → setCentralBankOf(signer) → registerCurrency
//	  → setCentralBankOf(cb) → grantRole(CENTRAL_BANK_ROLE, cb) → revokeRole(signer)
//
// Before this handover existed, cbAddress had to equal the hub signer, which is why every
// CB shared one hub key: the founding CB's. That made each sovereign W-token mintable by
// whoever held that single key.
//
// A fresh transactOpts is built before each transaction so nonces are managed
// automatically; each tx is waited to be mined before the next is submitted.
// Returns the deployed W-token address (hex) and the last transaction hash.
func (b *BesuClient) RegisterCurrency(ctx context.Context, tokenName, tokenSymbol, countryName, proposerCB, cbAddress string) (tokenAddr string, txHash string, err error) {
	if b.signer == nil {
		return "", "", ErrNoSigner
	}
	if strings.TrimSpace(b.cfg.CurrencyRegistryAddress) == "" {
		return "", "", errors.New("registry: CURRENCY_REGISTRY_ADDRESS is required for currency registration")
	}

	cb := common.HexToAddress(cbAddress)

	// 1) Deploy the W-token. admin == the signer; centralBank == the signer too, so the
	// signer can perform registerCurrency below. Authority moves to cb in step 4.
	opts1, err := b.transactOpts(ctx)
	if err != nil {
		return "", "", err
	}
	adminAddr := opts1.From
	wTokenAddr, deployTx, _, err := bindings.DeployTokenizedCentralBankMoney(opts1, b.client, tokenName, tokenSymbol, adminAddr, adminAddr)
	if err != nil {
		return "", "", fmt.Errorf("registry: deploy W-token: %w", err)
	}
	if err := b.waitMined(ctx, deployTx); err != nil {
		return wTokenAddr.Hex(), deployTx.Hash().Hex(), fmt.Errorf("registry: deploy W-token wait: %w", err)
	}

	// 2) Map the token to its interim central bank (the signer) on the IdentityRegistry.
	opts2, err := b.transactOpts(ctx)
	if err != nil {
		return wTokenAddr.Hex(), deployTx.Hash().Hex(), err
	}
	setCBTx, err := b.contract.SetCentralBankOf(opts2, wTokenAddr, adminAddr)
	if err != nil {
		return wTokenAddr.Hex(), deployTx.Hash().Hex(), fmt.Errorf("registry: setCentralBankOf tx: %w", err)
	}
	if err := b.waitMined(ctx, setCBTx); err != nil {
		return wTokenAddr.Hex(), setCBTx.Hash().Hex(), fmt.Errorf("registry: setCentralBankOf wait: %w", err)
	}

	// 3) Register the currency on the CurrencyRegistry (msg.sender must equal
	// getCentralBankOf(token), which step 2 made the signer).
	currencyReg, err := bindings.NewCurrencyRegistry(common.HexToAddress(b.cfg.CurrencyRegistryAddress), b.client)
	if err != nil {
		return wTokenAddr.Hex(), setCBTx.Hash().Hex(), fmt.Errorf("registry: binding CurrencyRegistry: %w", err)
	}
	opts3, err := b.transactOpts(ctx)
	if err != nil {
		return wTokenAddr.Hex(), setCBTx.Hash().Hex(), err
	}
	regTx, err := currencyReg.RegisterCurrency(opts3, tokenSymbol, countryName, wTokenAddr, proposerCB)
	if err != nil {
		return wTokenAddr.Hex(), setCBTx.Hash().Hex(), fmt.Errorf("registry: registerCurrency tx: %w", err)
	}
	if err := b.waitMined(ctx, regTx); err != nil {
		return wTokenAddr.Hex(), regTx.Hash().Hex(), fmt.Errorf("registry: registerCurrency wait: %w", err)
	}

	// 4) Hand issuance authority to the sovereign CB. Failing here leaves the currency
	// registered but issuable only by the hub — surfaced as an error so it is repaired
	// (EnsureCurrencyAuthority) rather than silently accepted.
	handoverTx, err := b.ensureCurrencyAuthority(ctx, wTokenAddr, cb)
	if err != nil {
		return wTokenAddr.Hex(), regTx.Hash().Hex(), err
	}
	if handoverTx != "" {
		return wTokenAddr.Hex(), handoverTx, nil
	}
	return wTokenAddr.Hex(), regTx.Hash().Hex(), nil
}

// EnsureCurrencyAuthority completes (or repairs) the handover of an already-registered
// currency's issuance authority to its central bank. Idempotent: it reads the current
// on-chain state and submits only the outstanding steps, so re-running a provisioning
// apply after a crash between registration and handover converges instead of skipping.
// Returns the last transaction hash, empty when nothing was outstanding.
func (b *BesuClient) EnsureCurrencyAuthority(ctx context.Context, tokenAddress, cbAddress string) (string, error) {
	if b.signer == nil {
		return "", ErrNoSigner
	}
	token := common.HexToAddress(tokenAddress)
	if token == (common.Address{}) {
		return "", errors.New("registry: token address is required for currency authority handover")
	}
	return b.ensureCurrencyAuthority(ctx, token, common.HexToAddress(cbAddress))
}

// ensureCurrencyAuthority reads the token's current authority and applies the outstanding
// steps of planCurrencyAuthority. The role is revoked from the signer only after the CB
// holds it, so the token is never left without an issuer.
func (b *BesuClient) ensureCurrencyAuthority(ctx context.Context, token, cb common.Address) (string, error) {
	wToken, err := bindings.NewTokenizedCentralBankMoney(token, b.client)
	if err != nil {
		return "", fmt.Errorf("registry: binding W-token %s: %w", token.Hex(), err)
	}
	role, err := wToken.CENTRALBANKROLE(&bind.CallOpts{Context: ctx})
	if err != nil {
		return "", fmt.Errorf("registry: read CENTRAL_BANK_ROLE: %w", err)
	}
	adminRole, err := wToken.DEFAULTADMINROLE(&bind.CallOpts{Context: ctx})
	if err != nil {
		return "", fmt.Errorf("registry: read DEFAULT_ADMIN_ROLE: %w", err)
	}
	signerHex, err := b.signer.SignerAddress(ctx)
	if err != nil {
		return "", fmt.Errorf("registry: resolve signer address: %w", err)
	}
	signerAddr := common.HexToAddress(signerHex)
	currentCBOf, err := b.contract.GetCentralBankOf(&bind.CallOpts{Context: ctx}, token)
	if err != nil {
		return "", fmt.Errorf("registry: read getCentralBankOf: %w", err)
	}
	cbHasRole, err := wToken.HasRole(&bind.CallOpts{Context: ctx}, role, cb)
	if err != nil {
		return "", fmt.Errorf("registry: read CB role: %w", err)
	}
	signerHasRole, err := wToken.HasRole(&bind.CallOpts{Context: ctx}, role, signerAddr)
	if err != nil {
		return "", fmt.Errorf("registry: read signer role: %w", err)
	}
	cbIsAdmin, err := wToken.HasRole(&bind.CallOpts{Context: ctx}, adminRole, cb)
	if err != nil {
		return "", fmt.Errorf("registry: read CB admin role: %w", err)
	}
	signerIsAdmin, err := wToken.HasRole(&bind.CallOpts{Context: ctx}, adminRole, signerAddr)
	if err != nil {
		return "", fmt.Errorf("registry: read signer admin role: %w", err)
	}

	plan := planCurrencyAuthority(signerAddr, cb, currentCBOf, cbHasRole, signerHasRole, cbIsAdmin, signerIsAdmin)
	if plan.Empty() {
		return "", nil
	}

	last := ""
	if plan.GrantCBRole {
		opts, oerr := b.transactOpts(ctx)
		if oerr != nil {
			return last, oerr
		}
		tx, gerr := wToken.GrantRole(opts, role, cb)
		if gerr != nil {
			return last, fmt.Errorf("registry: grant CENTRAL_BANK_ROLE to %s: %w", cb.Hex(), gerr)
		}
		if werr := b.waitMined(ctx, tx); werr != nil {
			return tx.Hash().Hex(), fmt.Errorf("registry: grant CENTRAL_BANK_ROLE wait: %w", werr)
		}
		last = tx.Hash().Hex()
	}
	if plan.GrantCBAdmin {
		opts, oerr := b.transactOpts(ctx)
		if oerr != nil {
			return last, oerr
		}
		tx, gerr := wToken.GrantRole(opts, adminRole, cb)
		if gerr != nil {
			return last, fmt.Errorf("registry: grant DEFAULT_ADMIN_ROLE to %s: %w", cb.Hex(), gerr)
		}
		if werr := b.waitMined(ctx, tx); werr != nil {
			return tx.Hash().Hex(), fmt.Errorf("registry: grant DEFAULT_ADMIN_ROLE wait: %w", werr)
		}
		last = tx.Hash().Hex()
	}
	if plan.SetCentralBankOf {
		opts, oerr := b.transactOpts(ctx)
		if oerr != nil {
			return last, oerr
		}
		tx, serr := b.contract.SetCentralBankOf(opts, token, cb)
		if serr != nil {
			return last, fmt.Errorf("registry: setCentralBankOf(%s) tx: %w", cb.Hex(), serr)
		}
		if werr := b.waitMined(ctx, tx); werr != nil {
			return tx.Hash().Hex(), fmt.Errorf("registry: setCentralBankOf(%s) wait: %w", cb.Hex(), werr)
		}
		last = tx.Hash().Hex()
	}
	if plan.RevokeSignerRole {
		opts, oerr := b.transactOpts(ctx)
		if oerr != nil {
			return last, oerr
		}
		tx, rerr := wToken.RevokeRole(opts, role, signerAddr)
		if rerr != nil {
			return last, fmt.Errorf("registry: revoke CENTRAL_BANK_ROLE from hub signer: %w", rerr)
		}
		if werr := b.waitMined(ctx, tx); werr != nil {
			return tx.Hash().Hex(), fmt.Errorf("registry: revoke CENTRAL_BANK_ROLE wait: %w", werr)
		}
		last = tx.Hash().Hex()
	}
	// LAST: this is the step that ends the hub's authority over the token, so everything above
	// must already be in place. Doing it earlier would leave the remaining steps unauthorized
	// and the token half-handed-over with no way to finish.
	if plan.RevokeSignerAdmin {
		opts, oerr := b.transactOpts(ctx)
		if oerr != nil {
			return last, oerr
		}
		tx, rerr := wToken.RevokeRole(opts, adminRole, signerAddr)
		if rerr != nil {
			return last, fmt.Errorf("registry: revoke DEFAULT_ADMIN_ROLE from hub signer: %w", rerr)
		}
		if werr := b.waitMined(ctx, tx); werr != nil {
			return tx.Hash().Hex(), fmt.Errorf("registry: revoke DEFAULT_ADMIN_ROLE wait: %w", werr)
		}
		last = tx.Hash().Hex()
	}
	return last, nil
}

// IsCurrencyRegistered reports whether a currency symbol is already registered
// on the CurrencyRegistry. getCurrency reverts (returns an error) when the
// symbol is absent, so any error is treated as "not registered". Returns an
// error only when the CurrencyRegistry address is not configured.
func (b *BesuClient) IsCurrencyRegistered(ctx context.Context, symbol string) (bool, error) {
	if strings.TrimSpace(b.cfg.CurrencyRegistryAddress) == "" {
		return false, errors.New("registry: CURRENCY_REGISTRY_ADDRESS is required for currency registration")
	}
	currencyReg, err := bindings.NewCurrencyRegistry(common.HexToAddress(b.cfg.CurrencyRegistryAddress), b.client)
	if err != nil {
		return false, fmt.Errorf("registry: binding CurrencyRegistry: %w", err)
	}
	entry, err := currencyReg.GetCurrency(&bind.CallOpts{Context: ctx}, symbol)
	if err != nil {
		// NotFound / revert → treated as not registered (not a hard error).
		return false, nil
	}
	// A populated token address indicates an existing registration.
	return entry.TokenAddress != (common.Address{}), nil
}

// CurrencyTokenAddress resolves a registered currency symbol to its W-token
// address ("0x…") via the CurrencyRegistry. Returns "" when absent/unconfigured.
func (b *BesuClient) CurrencyTokenAddress(ctx context.Context, symbol string) (string, error) {
	if strings.TrimSpace(b.cfg.CurrencyRegistryAddress) == "" {
		return "", errors.New("registry: CURRENCY_REGISTRY_ADDRESS is required")
	}
	addr, err := b.currencyTokenAddress(ctx, symbol)
	if err != nil {
		return "", err
	}
	return addr.Hex(), nil
}

// currencyTokenAddress resolves a registered currency symbol to its W-token
// address via the CurrencyRegistry (returns an error when absent).
func (b *BesuClient) currencyTokenAddress(ctx context.Context, symbol string) (common.Address, error) {
	currencyReg, err := bindings.NewCurrencyRegistry(common.HexToAddress(b.cfg.CurrencyRegistryAddress), b.client)
	if err != nil {
		return common.Address{}, fmt.Errorf("registry: binding CurrencyRegistry: %w", err)
	}
	entry, err := currencyReg.GetCurrency(&bind.CallOpts{Context: ctx}, symbol)
	if err != nil || entry.TokenAddress == (common.Address{}) {
		return common.Address{}, fmt.Errorf("registry: currency %q not registered", symbol)
	}
	return entry.TokenAddress, nil
}

// RegisterPair deploys the sovereign-pair AMM over the two ALREADY-registered
// W-tokens (resolved from the CurrencyRegistry by symbol) and registers the pair
// in the PairRegistry (proposePair + confirmPair → ACTIVE). The hub signer (hub
// admin, which holds the governance role) performs all txs, mirroring
// RegisterCurrency — the CBs never touch the hub chain. Returns the deployed AMM
// address + last tx hash. Idempotent: an already-ACTIVE pair is a no-op.
func (b *BesuClient) RegisterPair(ctx context.Context, symbolA, symbolB, pairID string) (ammAddr string, txHash string, err error) {
	if b.signer == nil {
		return "", "", ErrNoSigner
	}
	if strings.TrimSpace(b.cfg.PairRegistryAddress) == "" {
		return "", "", errors.New("registry: PAIR_REGISTRY_ADDRESS is required for pair registration")
	}
	if strings.TrimSpace(b.cfg.CurrencyRegistryAddress) == "" {
		return "", "", errors.New("registry: CURRENCY_REGISTRY_ADDRESS is required for pair registration")
	}

	tokenA, err := b.currencyTokenAddress(ctx, symbolA)
	if err != nil {
		return "", "", err
	}
	tokenB, err := b.currencyTokenAddress(ctx, symbolB)
	if err != nil {
		return "", "", err
	}

	pairReg, err := bindings.NewPairRegistry(common.HexToAddress(b.cfg.PairRegistryAddress), b.client)
	if err != nil {
		return "", "", fmt.Errorf("registry: binding PairRegistry: %w", err)
	}

	// 1) Deploy the AMM bound to the two W-tokens + the hub IdentityRegistry.
	opts1, err := b.transactOpts(ctx)
	if err != nil {
		return "", "", err
	}
	amm, deployTx, _, err := bindings.DeployAutomatedMarketMaker(opts1, b.client, tokenA, tokenB, common.HexToAddress(b.cfg.RegistryAddress))
	if err != nil {
		return "", "", fmt.Errorf("registry: deploy AMM: %w", err)
	}
	if err := b.waitMined(ctx, deployTx); err != nil {
		return amm.Hex(), deployTx.Hash().Hex(), fmt.Errorf("registry: deploy AMM wait: %w", err)
	}

	// 2) proposePair / confirmPair are the two sovereigns' own acts: the PairRegistry
	// requires msg.sender == getCentralBankOf(tokenA) to propose and
	// getCentralBankOf(tokenB) to confirm. Once each currency's issuance authority rests
	// with its own central bank, the hub is neither — it deploys the AMM (neutral
	// infrastructure) and stops there, leaving each CB to sign its own side from its
	// governance portal (POST /api/v2/amm/pairs/{propose,confirm}).
	//
	// The hub still performs both when it genuinely is the central bank of both tokens
	// (single-entity stacks), so those setups keep working unchanged.
	signerHex, err := b.signer.SignerAddress(ctx)
	if err != nil {
		return amm.Hex(), deployTx.Hash().Hex(), fmt.Errorf("registry: resolve signer address: %w", err)
	}
	signerAddr := common.HexToAddress(signerHex)
	cbOfA, err := b.contract.GetCentralBankOf(&bind.CallOpts{Context: ctx}, tokenA)
	if err != nil {
		return amm.Hex(), deployTx.Hash().Hex(), fmt.Errorf("registry: read getCentralBankOf(tokenA): %w", err)
	}
	cbOfB, err := b.contract.GetCentralBankOf(&bind.CallOpts{Context: ctx}, tokenB)
	if err != nil {
		return amm.Hex(), deployTx.Hash().Hex(), fmt.Errorf("registry: read getCentralBankOf(tokenB): %w", err)
	}
	if signerAddr != cbOfA || signerAddr != cbOfB {
		// Not a silent partial success: the AMM exists and the pair does not, and the caller
		// is told exactly which acts remain and who owes them.
		return amm.Hex(), deployTx.Hash().Hex(), &PairAwaitsSovereignsError{
			PairID:    pairID,
			AMM:       amm.Hex(),
			ProposeBy: cbOfA.Hex(),
			ConfirmBy: cbOfB.Hex(),
		}
	}

	opts2, err := b.transactOpts(ctx)
	if err != nil {
		return amm.Hex(), deployTx.Hash().Hex(), err
	}
	propTx, err := pairReg.ProposePair(opts2, pairID, tokenA, tokenB, amm)
	if err != nil {
		return amm.Hex(), deployTx.Hash().Hex(), fmt.Errorf("registry: proposePair tx: %w", err)
	}
	if err := b.waitMined(ctx, propTx); err != nil {
		return amm.Hex(), propTx.Hash().Hex(), fmt.Errorf("registry: proposePair wait: %w", err)
	}

	// 3) confirmPair → ACTIVE.
	opts3, err := b.transactOpts(ctx)
	if err != nil {
		return amm.Hex(), propTx.Hash().Hex(), err
	}
	confTx, err := pairReg.ConfirmPair(opts3, pairID)
	if err != nil {
		return amm.Hex(), propTx.Hash().Hex(), fmt.Errorf("registry: confirmPair tx: %w", err)
	}
	if err := b.waitMined(ctx, confTx); err != nil {
		return amm.Hex(), confTx.Hash().Hex(), fmt.Errorf("registry: confirmPair wait: %w", err)
	}

	return amm.Hex(), confTx.Hash().Hex(), nil
}

// PairAwaitsSovereignsError reports that the corridor's AMM was deployed but the pair
// itself must be proposed and confirmed by the two issuing central banks, because the hub
// is the central bank of neither token. It carries the addresses that owe each act so the
// caller can route the request instead of guessing.
type PairAwaitsSovereignsError struct {
	PairID    string
	AMM       string
	ProposeBy string
	ConfirmBy string
}

func (e *PairAwaitsSovereignsError) Error() string {
	return fmt.Sprintf(
		"pair %s: AMM deployed at %s, but proposePair must be signed by the central bank of token A (%s) and confirmPair by the central bank of token B (%s) — the hub is neither",
		e.PairID, e.AMM, e.ProposeBy, e.ConfirmBy,
	)
}

// IsPairRegistered reports whether a pair already exists (any status) in the
// PairRegistry. Used for idempotency. Returns an error only when the
// PairRegistry address is not configured.
func (b *BesuClient) IsPairRegistered(ctx context.Context, pairID string) (bool, error) {
	if strings.TrimSpace(b.cfg.PairRegistryAddress) == "" {
		return false, errors.New("registry: PAIR_REGISTRY_ADDRESS is required for pair registration")
	}
	pairReg, err := bindings.NewPairRegistry(common.HexToAddress(b.cfg.PairRegistryAddress), b.client)
	if err != nil {
		return false, fmt.Errorf("registry: binding PairRegistry: %w", err)
	}
	entry, err := pairReg.GetPair(&bind.CallOpts{Context: ctx}, pairID)
	if err != nil {
		return false, nil // NotFound / revert → not registered
	}
	return entry.AmmAddress != (common.Address{}), nil
}

// waitMined blocks until the transaction is included in a block. Returns an
// error if the transaction reverted (receipt status != 1) or the context
// expires before the tx is mined.
func (b *BesuClient) waitMined(ctx context.Context, tx *types.Transaction) error {
	receipt, err := bind.WaitMined(ctx, b.client, tx)
	if err != nil {
		return fmt.Errorf("waiting for tx %s: %w", tx.Hash().Hex(), err)
	}
	if receipt.Status == 0 {
		return fmt.Errorf("tx %s reverted (status=0)", tx.Hash().Hex())
	}
	log.Printf("registry: tx %s mined in block %s (gas=%d)", tx.Hash().Hex(), receipt.BlockNumber, receipt.GasUsed)
	return nil
}

// CanTransact returns true if the address is Verified and has a non-NONE role.
func (b *BesuClient) CanTransact(ctx context.Context, address string) (bool, error) {
	account := common.HexToAddress(address)
	result, err := b.contract.CanTransact(&bind.CallOpts{Context: ctx}, account)
	if err != nil {
		return false, fmt.Errorf("registry: canTransact: %w", err)
	}
	return result, nil
}

// IsWhitelisted returns true if the address has Verified KYC status.
func (b *BesuClient) IsWhitelisted(ctx context.Context, address string) (bool, error) {
	account := common.HexToAddress(address)
	result, err := b.contract.IsWhitelisted(&bind.CallOpts{Context: ctx}, account)
	if err != nil {
		return false, fmt.Errorf("registry: isWhitelisted: %w", err)
	}
	return result, nil
}

// GetParticipant returns the full on-chain participant profile.
func (b *BesuClient) GetParticipant(ctx context.Context, address string) (OnChainParticipant, error) {
	account := common.HexToAddress(address)
	p, err := b.contract.GetParticipant(&bind.CallOpts{Context: ctx}, account)
	if err != nil {
		return OnChainParticipant{}, fmt.Errorf("registry: getParticipant: %w", err)
	}
	return OnChainParticipant{
		LegalName:       p.LegalName,
		Role:            p.Role,
		Status:          p.Status,
		ZkPointer:       p.ZkPointer,
		CertFingerprint: p.CertFingerprint,
		LastUpdate:      p.LastUpdate,
	}, nil
}

// GetCertFingerprint returns the certificate fingerprint stored on-chain.
func (b *BesuClient) GetCertFingerprint(ctx context.Context, address string) ([32]byte, error) {
	account := common.HexToAddress(address)
	fp, err := b.contract.GetCertFingerprint(&bind.CallOpts{Context: ctx}, account)
	if err != nil {
		return [32]byte{}, fmt.Errorf("registry: getCertFingerprint: %w", err)
	}
	return fp, nil
}

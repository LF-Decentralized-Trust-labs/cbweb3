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

// RegisterCurrency deploys a founding central bank's bridge token (W-token),
// maps that token to the central bank on the IdentityRegistry, and registers
// the sovereign currency on the CurrencyRegistry. All three transactions are
// signed by this client's signer, which in local is the central bank == the hub
// admin (holding DEFAULT_ADMIN_ROLE and being the token's central bank).
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

	// 1) Deploy the W-token. admin == the signer (hub admin); centralBank == cb.
	opts1, err := b.transactOpts(ctx)
	if err != nil {
		return "", "", err
	}
	adminAddr := opts1.From
	wTokenAddr, deployTx, _, err := bindings.DeployTokenizedCentralBankMoney(opts1, b.client, tokenName, tokenSymbol, adminAddr, cb)
	if err != nil {
		return "", "", fmt.Errorf("registry: deploy W-token: %w", err)
	}
	if err := b.waitMined(ctx, deployTx); err != nil {
		return wTokenAddr.Hex(), deployTx.Hash().Hex(), fmt.Errorf("registry: deploy W-token wait: %w", err)
	}

	// 2) Map the token to its central bank on the IdentityRegistry.
	opts2, err := b.transactOpts(ctx)
	if err != nil {
		return wTokenAddr.Hex(), deployTx.Hash().Hex(), err
	}
	setCBTx, err := b.contract.SetCentralBankOf(opts2, wTokenAddr, cb)
	if err != nil {
		return wTokenAddr.Hex(), deployTx.Hash().Hex(), fmt.Errorf("registry: setCentralBankOf tx: %w", err)
	}
	if err := b.waitMined(ctx, setCBTx); err != nil {
		return wTokenAddr.Hex(), setCBTx.Hash().Hex(), fmt.Errorf("registry: setCentralBankOf wait: %w", err)
	}

	// 3) Register the currency on the CurrencyRegistry (msg.sender must equal
	// getCentralBankOf(token); in local the signer == cb).
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

	return wTokenAddr.Hex(), regTx.Hash().Hex(), nil
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

	// 2) proposePair (records tokenA/tokenB/amm under pairID).
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

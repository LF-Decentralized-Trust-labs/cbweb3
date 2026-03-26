package registry

import (
	"context"
	"errors"
	"fmt"
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

	return &bind.TransactOpts{
		Context: ctx,
		From:    common.HexToAddress(signerAddr),
		Signer: func(_ common.Address, tx *types.Transaction) (*types.Transaction, error) {
			return signer.SignTx(ctx, tx, chainID)
		},
	}, nil
}

// RegisterParticipant registers a new participant on the IdentityRegistry contract.
func (b *BesuClient) RegisterParticipant(ctx context.Context, wallet, name, role string, zkPointer [32]byte) (string, error) {
	opts, err := b.transactOpts(ctx)
	if err != nil {
		return "", err
	}

	solidityRole := RoleToSolidityEnum(role)
	account := common.HexToAddress(wallet)

	tx, err := b.contract.RegisterParticipant(opts, account, name, solidityRole, zkPointer)
	if err != nil {
		return "", fmt.Errorf("registry: registerParticipant tx: %w", err)
	}
	return tx.Hash().Hex(), nil
}

// UpdateStatus changes the KYC status of a participant on-chain.
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
	return tx.Hash().Hex(), nil
}

// SetCertFingerprint stores a certificate fingerprint on-chain.
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
	return tx.Hash().Hex(), nil
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

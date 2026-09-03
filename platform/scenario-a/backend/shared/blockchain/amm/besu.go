// SPDX-License-Identifier: Apache-2.0

package amm

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

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/amm/bindings"
	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/registry"
)

// BesuConfig holds the configuration to connect to a Besu node and drive the
// AutomatedMarketMaker circuit breaker.
//
// Environment variables used by the compliance service:
//
//	BLOCKCHAIN_CLIENT              — must be "besu" to enable the on-chain breaker
//	BESU_RPC_URL                   — e.g. "http://besu-node:8545"
//	AMM_ADDRESS                    — deployed AutomatedMarketMaker address "0x..."
//	CB_PRIVATE_KEY                 — Central Bank governance key (hex)
//	BESU_CHAIN_ID                  — Besu network chain ID (default: 1337)
type BesuConfig struct {
	RPCURL         string
	AMMAddress     string
	ChainID        int64
	RequestTimeout time.Duration
}

// BesuBreaker implements Breaker using go-ethereum and the abigen-generated
// AutomatedMarketMaker bindings. It reuses the registry package's TransactionSigner
// so Central Banks share a single StaticKeySigner across contracts.
type BesuBreaker struct {
	cfg      BesuConfig
	signer   registry.TransactionSigner
	client   *ethclient.Client
	contract *bindings.AutomatedMarketMaker
}

// NewBesuBreaker connects to the given RPC URL and binds the AMM at AMMAddress.
// A signer is required — every breaker operation is a state-changing transaction.
func NewBesuBreaker(cfg BesuConfig, signer registry.TransactionSigner) (*BesuBreaker, error) {
	if strings.TrimSpace(cfg.RPCURL) == "" {
		return nil, errors.New("amm: BESU_RPC_URL is required")
	}
	if strings.TrimSpace(cfg.AMMAddress) == "" {
		return nil, errors.New("amm: AMM_ADDRESS is required")
	}
	if signer == nil {
		return nil, ErrNoSigner
	}

	client, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("amm: connecting to Besu node: %w", err)
	}

	contract, err := bindings.NewAutomatedMarketMaker(common.HexToAddress(cfg.AMMAddress), client)
	if err != nil {
		return nil, fmt.Errorf("amm: binding AutomatedMarketMaker contract: %w", err)
	}

	return &BesuBreaker{cfg: cfg, signer: signer, client: client, contract: contract}, nil
}

// transactOpts builds bind.TransactOpts from the TransactionSigner (mirrors the
// registry package's pattern: explicit From + GasPrice, signing via the closure).
func (b *BesuBreaker) transactOpts(ctx context.Context) (*bind.TransactOpts, error) {
	if b.signer == nil {
		return nil, ErrNoSigner
	}

	signerAddr, err := b.signer.SignerAddress(ctx)
	if err != nil {
		return nil, fmt.Errorf("amm: resolving signer address: %w", err)
	}

	chainID := big.NewInt(b.cfg.ChainID)
	signer := b.signer

	gasPrice, err := b.client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("amm: fetching gas price: %w", err)
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

// Pause engages the breaker (1-of-N). Blocks until mined.
func (b *BesuBreaker) Pause(ctx context.Context, reason string) (string, error) {
	opts, err := b.transactOpts(ctx)
	if err != nil {
		return "", err
	}
	tx, err := b.contract.Pause(opts, reason)
	if err != nil {
		return "", fmt.Errorf("amm: pause tx: %w", err)
	}
	if err := b.waitMined(ctx, tx); err != nil {
		return tx.Hash().Hex(), fmt.Errorf("amm: pause wait: %w", err)
	}
	return tx.Hash().Hex(), nil
}

// ResumeVote signs the active resume proposal for the current pause epoch, or opens
// a new one if none exists. Gas estimation surfaces contract reverts (e.g. this
// signer already voted, or the proposal was superseded) as an error before sending.
func (b *BesuBreaker) ResumeVote(ctx context.Context) (string, error) {
	paused, err := b.IsPaused(ctx)
	if err != nil {
		return "", err
	}
	if !paused {
		return "", ErrNotPaused
	}

	proposalID, found, err := b.findActiveProposalID(ctx)
	if err != nil {
		return "", err
	}

	opts, err := b.transactOpts(ctx)
	if err != nil {
		return "", err
	}

	var tx *types.Transaction
	if found {
		// Add this signer's signature to the existing proposal; may reach quorum.
		tx, err = b.contract.SignResume(opts, proposalID)
		if err != nil {
			return "", fmt.Errorf("amm: signResume tx: %w", err)
		}
	} else {
		// No proposal yet this epoch — open one (counts as the first signature).
		tx, err = b.contract.ProposeResume(opts)
		if err != nil {
			return "", fmt.Errorf("amm: proposeResume tx: %w", err)
		}
	}

	if err := b.waitMined(ctx, tx); err != nil {
		return tx.Hash().Hex(), fmt.Errorf("amm: resume vote wait: %w", err)
	}
	return tx.Hash().Hex(), nil
}

// IsPaused reads the on-chain breaker state.
func (b *BesuBreaker) IsPaused(ctx context.Context) (bool, error) {
	paused, err := b.contract.IsPaused(&bind.CallOpts{Context: ctx})
	if err != nil {
		return false, fmt.Errorf("amm: isPaused: %w", err)
	}
	return paused, nil
}

// findActiveProposalID returns the most recent resume proposal opened at or after
// the latest pause. That proposal necessarily belongs to the current pause epoch,
// so signing it can reach quorum without hitting the epoch-expiry guard. Returns
// found=false when no proposal has been opened since the last pause.
func (b *BesuBreaker) findActiveProposalID(ctx context.Context) ([32]byte, bool, error) {
	pauseBlock, ok, err := b.latestPauseBlock(ctx)
	if err != nil {
		return [32]byte{}, false, err
	}
	if !ok {
		return [32]byte{}, false, nil
	}

	it, err := b.contract.FilterLogResumeProposed(&bind.FilterOpts{Start: pauseBlock, Context: ctx}, nil, nil)
	if err != nil {
		return [32]byte{}, false, fmt.Errorf("amm: filtering resume proposals: %w", err)
	}
	defer it.Close()

	var last [32]byte
	found := false
	for it.Next() {
		last = it.Event.ProposalId
		found = true
	}
	if err := it.Error(); err != nil {
		return [32]byte{}, false, fmt.Errorf("amm: iterating resume proposals: %w", err)
	}
	return last, found, nil
}

// latestPauseBlock returns the block number of the most recent LogCircuitBreakerPaused.
func (b *BesuBreaker) latestPauseBlock(ctx context.Context) (uint64, bool, error) {
	it, err := b.contract.FilterLogCircuitBreakerPaused(&bind.FilterOpts{Start: 0, Context: ctx}, nil)
	if err != nil {
		return 0, false, fmt.Errorf("amm: filtering pause events: %w", err)
	}
	defer it.Close()

	var block uint64
	ok := false
	for it.Next() {
		block = it.Event.Raw.BlockNumber
		ok = true
	}
	if err := it.Error(); err != nil {
		return 0, false, fmt.Errorf("amm: iterating pause events: %w", err)
	}
	return block, ok, nil
}

// waitMined blocks until the tx is included and reverts (status 0) are surfaced.
func (b *BesuBreaker) waitMined(ctx context.Context, tx *types.Transaction) error {
	receipt, err := bind.WaitMined(ctx, b.client, tx)
	if err != nil {
		return fmt.Errorf("waiting for tx %s: %w", tx.Hash().Hex(), err)
	}
	if receipt.Status == 0 {
		return fmt.Errorf("tx %s reverted (status=0)", tx.Hash().Hex())
	}
	log.Printf("amm: tx %s mined in block %s (gas=%d)", tx.Hash().Hex(), receipt.BlockNumber, receipt.GasUsed)
	return nil
}

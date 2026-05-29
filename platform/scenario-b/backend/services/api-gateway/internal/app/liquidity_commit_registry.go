// Package app — LiquidityCommitRegistry EVM adapter.
// Wraps the on-chain LiquidityCommitRegistry contract (contracts/src/LiquidityCommitRegistry.sol)
// for use by sovereign_liquidity_service.go and the CommitLiquidity handler
// (007-bridge-based-cb-liquidity).
package app

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/evm"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// liquidityCommitRegistryABI is the minimal ABI for LiquidityCommitRegistry.sol.
// Keep in sync with contracts/src/LiquidityCommitRegistry.sol.
//
// CommitSide enum: A=0, B=1 (uint8)
// CommitStatus enum: PENDING=0, MATCHED=1, EXPIRED=2, CANCELLED=3 (uint8)
const liquidityCommitRegistryABI = `[
{"type":"function","name":"registerCommit","stateMutability":"nonpayable","inputs":[
  {"name":"poolPair","type":"string"},
  {"name":"side","type":"uint8"},
  {"name":"amount","type":"uint256"},
  {"name":"wTokenAddress","type":"address"}
],"outputs":[
  {"name":"commitId","type":"bytes32"}
]},
{"type":"function","name":"cancelCommit","stateMutability":"nonpayable","inputs":[
  {"name":"commitId","type":"bytes32"}
],"outputs":[]},
{"type":"function","name":"expireCommit","stateMutability":"nonpayable","inputs":[
  {"name":"commitId","type":"bytes32"}
],"outputs":[]},
{"type":"function","name":"getCommit","stateMutability":"view","inputs":[
  {"name":"commitId","type":"bytes32"}
],"outputs":[
  {"name":"signer","type":"address"},
  {"name":"wTokenAddr","type":"address"},
  {"name":"amount","type":"uint256"},
  {"name":"expiresAt","type":"uint256"},
  {"name":"status","type":"uint8"},
  {"name":"side","type":"uint8"}
]},
{"type":"function","name":"getPendingCommit","stateMutability":"view","inputs":[
  {"name":"poolPair","type":"string"},
  {"name":"side","type":"uint8"}
],"outputs":[
  {"name":"commitId","type":"bytes32"}
]},
{"type":"event","name":"CommitRegistered","anonymous":false,"inputs":[
  {"name":"commitId","type":"bytes32","indexed":true},
  {"name":"poolPair","type":"string","indexed":false},
  {"name":"side","type":"uint8","indexed":false},
  {"name":"signer","type":"address","indexed":true},
  {"name":"wTokenAddr","type":"address","indexed":false},
  {"name":"amount","type":"uint256","indexed":false},
  {"name":"expiresAt","type":"uint256","indexed":false}
]},
{"type":"event","name":"CommitMatched","anonymous":false,"inputs":[
  {"name":"poolPair","type":"string","indexed":true},
  {"name":"commitIdA","type":"bytes32","indexed":false},
  {"name":"signerA","type":"address","indexed":false},
  {"name":"amountA","type":"uint256","indexed":false},
  {"name":"commitIdB","type":"bytes32","indexed":false},
  {"name":"signerB","type":"address","indexed":false},
  {"name":"amountB","type":"uint256","indexed":false}
]},
{"type":"event","name":"CommitExpired","anonymous":false,"inputs":[
  {"name":"commitId","type":"bytes32","indexed":true},
  {"name":"poolPair","type":"string","indexed":false},
  {"name":"side","type":"uint8","indexed":false}
]},
{"type":"event","name":"CommitCancelled","anonymous":false,"inputs":[
  {"name":"commitId","type":"bytes32","indexed":true}
]}
]`

// LiquidityCommitRegistryClient is an EVM client for LiquidityCommitRegistry.sol.
type LiquidityCommitRegistryClient struct {
	contract common.Address
	ec       *ethclient.Client
	parsed   abi.ABI
	signer   *evm.Signer
	timeout  time.Duration
}

// LiquidityCommitRegistryConfig holds connection parameters for the adapter.
type LiquidityCommitRegistryConfig struct {
	RPCURL          string
	ContractAddress string
	ChainID         int64
	PrivateKeyHex   string
	Timeout         time.Duration
}

// NewLiquidityCommitRegistryClient constructs a LiquidityCommitRegistryClient.
func NewLiquidityCommitRegistryClient(ctx context.Context, cfg LiquidityCommitRegistryConfig) (*LiquidityCommitRegistryClient, error) {
	if cfg.ContractAddress == "" {
		return nil, fmt.Errorf("LiquidityCommitRegistryConfig: ContractAddress is required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	ec, err := evm.Dial(ctx, cfg.RPCURL, cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("liquidity commit registry: dial: %w", err)
	}
	parsed, err := evm.ParseABI(liquidityCommitRegistryABI)
	if err != nil {
		ec.Close()
		return nil, fmt.Errorf("liquidity commit registry: parse ABI: %w", err)
	}
	c := &LiquidityCommitRegistryClient{
		contract: common.HexToAddress(cfg.ContractAddress),
		ec:       ec,
		parsed:   parsed,
		timeout:  cfg.Timeout,
	}
	if cfg.PrivateKeyHex != "" {
		signer, sigErr := evm.NewSigner(cfg.PrivateKeyHex, big.NewInt(cfg.ChainID))
		if sigErr != nil {
			ec.Close()
			return nil, fmt.Errorf("liquidity commit registry: signer: %w", sigErr)
		}
		c.signer = signer
	}
	return c, nil
}

// Close releases the underlying RPC connection.
func (c *LiquidityCommitRegistryClient) Close() {
	if c.ec != nil {
		c.ec.Close()
	}
}

// RegisterCommit submits a registerCommit transaction on-chain.
//
// Parameters:
//
//	poolPair      — pool pair identifier (e.g. "W-BRL-ARS")
//	side          — 0 for CommitSide.A, 1 for CommitSide.B
//	amount        — deposit amount in wei
//	wTokenAddress — W-tCeBM contract address on the Hub
//
// Returns the commitId ([32]byte) assigned on-chain.
func (c *LiquidityCommitRegistryClient) RegisterCommit(
	ctx context.Context,
	poolPair string,
	side uint8,
	amount *big.Int,
	wTokenAddress common.Address,
) ([32]byte, error) {
	if c.signer == nil {
		return [32]byte{}, fmt.Errorf("liquidity commit registry: RegisterCommit requires a signing key")
	}

	txHash, err := evm.SubmitTx(
		ctx, c.ec, c.signer, c.contract, c.parsed,
		"registerCommit",
		poolPair,
		side,
		amount,
		wTokenAddress,
	)
	if err != nil {
		return [32]byte{}, fmt.Errorf("liquidity commit registry registerCommit: %w", err)
	}

	// Fetch the transaction receipt to extract the commitId from the CommitRegistered event.
	tCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	txHashBytes := common.HexToHash(txHash)
	receipt, err := c.ec.TransactionReceipt(tCtx, txHashBytes)
	if err != nil {
		return [32]byte{}, fmt.Errorf("liquidity commit registry: receipt for %s: %w", txHash, err)
	}

	// Parse CommitRegistered from receipt logs.
	event := c.parsed.Events["CommitRegistered"]
	for _, log := range receipt.Logs {
		if len(log.Topics) < 1 {
			continue
		}
		if log.Topics[0] != event.ID {
			continue
		}
		// commitId is the first indexed topic (Topics[1]).
		if len(log.Topics) >= 2 {
			var commitId [32]byte
			copy(commitId[:], log.Topics[1].Bytes())
			return commitId, nil
		}
	}

	return [32]byte{}, fmt.Errorf("liquidity commit registry: CommitRegistered event not found in tx %s", txHash)
}

// CancelCommit submits a cancelCommit transaction on-chain.
func (c *LiquidityCommitRegistryClient) CancelCommit(ctx context.Context, commitId [32]byte) error {
	if c.signer == nil {
		return fmt.Errorf("liquidity commit registry: CancelCommit requires a signing key")
	}
	_, err := evm.SubmitTx(ctx, c.ec, c.signer, c.contract, c.parsed, "cancelCommit", commitId)
	if err != nil {
		return fmt.Errorf("liquidity commit registry cancelCommit: %w", err)
	}
	return nil
}

// GetPendingCommit returns the currently PENDING commitId for a (poolPair, side).
// Returns a zero [32]byte if no PENDING commit exists.
func (c *LiquidityCommitRegistryClient) GetPendingCommit(ctx context.Context, poolPair string, side uint8) ([32]byte, error) {
	input, err := c.parsed.Pack("getPendingCommit", poolPair, side)
	if err != nil {
		return [32]byte{}, fmt.Errorf("liquidity commit registry getPendingCommit pack: %w", err)
	}
	msg := ethereum.CallMsg{To: &c.contract, Data: input}
	raw, err := c.ec.CallContract(ctx, msg, nil)
	if err != nil {
		return [32]byte{}, fmt.Errorf("liquidity commit registry getPendingCommit call: %w", err)
	}

	method := c.parsed.Methods["getPendingCommit"]
	values, err := method.Outputs.Unpack(raw)
	if err != nil {
		return [32]byte{}, fmt.Errorf("liquidity commit registry getPendingCommit unpack: %w", err)
	}
	if len(values) == 0 {
		return [32]byte{}, nil
	}
	commitId, ok := values[0].([32]byte)
	if !ok {
		return [32]byte{}, fmt.Errorf("liquidity commit registry getPendingCommit: unexpected type %T", values[0])
	}
	return commitId, nil
}

// CommitIDToHex converts a [32]byte commitId to a 0x-prefixed hex string.
func CommitIDToHex(id [32]byte) string {
	return "0x" + hex.EncodeToString(id[:])
}

// CommitIDFromHex converts a 0x-prefixed hex string back to [32]byte.
// Returns an error if the string is not a valid 32-byte hex value.
func CommitIDFromHex(s string) ([32]byte, error) {
	s = strings.TrimPrefix(s, "0x")
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 32 {
		return [32]byte{}, fmt.Errorf("liquidity commit registry: invalid commitId hex: %q", s)
	}
	var id [32]byte
	copy(id[:], b)
	return id, nil
}

// lcrHandlerAdapter wraps LiquidityCommitRegistryClient to satisfy handlers.OnChainCommitRegistrarIface.
// It bridges the type mismatch between [32]byte/common.Address and []byte/string.
type lcrHandlerAdapter struct {
	c *LiquidityCommitRegistryClient
}

// RegisterCommit implements handlers.OnChainCommitRegistrarIface.
func (a *lcrHandlerAdapter) RegisterCommit(ctx context.Context, poolPair string, side uint8, amount *big.Int, wTokenAddress string) ([]byte, error) {
	addr := common.HexToAddress(wTokenAddress)
	commitID, err := a.c.RegisterCommit(ctx, poolPair, side, amount, addr)
	if err != nil {
		return nil, err
	}
	return commitID[:], nil
}

// Package spokebridge provides an EVM client for the SpokeBridge contract deployed on a
// Spoke network. It wraps go-ethereum/ethclient with a minimal embedded ABI so the
// Payment Orchestrator can call lock/release/getLock without requiring abigen-generated
// bindings (T058).
package spokebridge

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/evm"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ABIJSON is the minimal ISpokeBridge ABI used by the Go client. Keep signatures in sync
// with contracts/src/interfaces/ISpokeBridge.sol.
const ABIJSON = `[
{"type":"function","name":"lock","stateMutability":"nonpayable","inputs":[
  {"name":"token","type":"address"},
  {"name":"amount","type":"uint256"},
  {"name":"txId","type":"bytes32"}
],"outputs":[]},
{"type":"function","name":"release","stateMutability":"nonpayable","inputs":[
  {"name":"txId","type":"bytes32"}
],"outputs":[]},
{"type":"function","name":"getLock","stateMutability":"view","inputs":[
  {"name":"txId","type":"bytes32"}
],"outputs":[
  {"name":"sender","type":"address"},
  {"name":"token","type":"address"},
  {"name":"amount","type":"uint256"},
  {"name":"released","type":"bool"}
]}
]`

// LockRequest describes a lock operation on a Spoke.
type LockRequest struct {
	TokenAddress string
	Amount       string
	TxID         [32]byte
}

// LockResult holds the result of a successful lock call.
type LockResult struct {
	TxHash    string
	LockProof string
}

// UnlockRequest describes a release operation on a Spoke.
type UnlockRequest struct {
	TxID [32]byte
}

// UnlockResult holds the result of a successful release call.
type UnlockResult struct {
	TxHash string
}

// LockStatus mirrors the struct returned by SpokeBridge.getLock().
type LockStatus struct {
	Sender   common.Address
	Token    common.Address
	Amount   *big.Int
	Released bool
}

// Client is the EVM client for the SpokeBridge contract.
type Client struct {
	contract common.Address
	ec       *ethclient.Client
	abi      abi.ABI
	signer   *evm.Signer
	timeout  time.Duration
}

// Config holds the connection parameters for the SpokeBridge client.
type Config struct {
	RPCURL          string
	ContractAddress string
	ChainID         int64
	PrivateKeyHex   string
	Timeout         time.Duration
}

// NewClient opens an RPC connection to the Spoke and parses the ABI.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.ContractAddress == "" {
		return nil, errors.New("ContractAddress is required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	ec, err := evm.Dial(ctx, cfg.RPCURL, cfg.Timeout)
	if err != nil {
		return nil, err
	}
	parsed, err := evm.ParseABI(ABIJSON)
	if err != nil {
		return nil, fmt.Errorf("parse SpokeBridge ABI: %w", err)
	}
	client := &Client{
		contract: common.HexToAddress(cfg.ContractAddress),
		ec:       ec,
		abi:      parsed,
		timeout:  cfg.Timeout,
	}
	if cfg.PrivateKeyHex != "" {
		signer, err := evm.NewSigner(cfg.PrivateKeyHex, big.NewInt(cfg.ChainID))
		if err != nil {
			return nil, err
		}
		client.signer = signer
	}
	return client, nil
}

// Close releases the underlying RPC connection.
func (c *Client) Close() {
	if c.ec != nil {
		c.ec.Close()
	}
}

// LockAsset locks native assets on a Spoke, producing a canonical LockResult.
func (c *Client) LockAsset(ctx context.Context, req LockRequest) (*LockResult, error) {
	if c.signer == nil {
		return nil, errors.New("spokebridge: lock requires a signing key")
	}
	amount, ok := new(big.Int).SetString(strings.TrimSpace(req.Amount), 10)
	if !ok {
		return nil, fmt.Errorf("invalid lock amount %q", req.Amount)
	}
	txHash, err := evm.SubmitTx(ctx, c.ec, c.signer, c.contract, c.abi,
		"lock",
		common.HexToAddress(req.TokenAddress),
		amount,
		req.TxID,
	)
	if err != nil {
		return nil, err
	}
	return &LockResult{TxHash: txHash, LockProof: txHash}, nil
}

// UnlockAsset releases previously locked assets on a Spoke (GOVERNANCE-only on-chain).
func (c *Client) UnlockAsset(ctx context.Context, req UnlockRequest) (*UnlockResult, error) {
	if c.signer == nil {
		return nil, errors.New("spokebridge: release requires a signing key")
	}
	txHash, err := evm.SubmitTx(ctx, c.ec, c.signer, c.contract, c.abi, "release", req.TxID)
	if err != nil {
		return nil, err
	}
	return &UnlockResult{TxHash: txHash}, nil
}

// GetLock returns the current status of a locked transaction.
func (c *Client) GetLock(ctx context.Context, txID [32]byte) (*LockStatus, error) {
	var (
		sender   common.Address
		token    common.Address
		amount   = new(big.Int)
		released bool
	)
	if err := evm.Call(ctx, c.ec, c.contract, c.abi, "getLock",
		[]interface{}{txID}, &sender, &token, amount, &released); err != nil {
		return nil, err
	}
	return &LockStatus{Sender: sender, Token: token, Amount: amount, Released: released}, nil
}

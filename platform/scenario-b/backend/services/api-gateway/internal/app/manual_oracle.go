// Package app — ManualOracle EVM adapter (read-only).
// Wraps the Hub ManualOracle contract (contracts/src/ManualOracle.sol) so the
// api-gateway can read FX rates and suggest a counterpart matching deposit amount
// during cooperative liquidity coordination. The oracle is system-managed: only the
// platform operator holds the rate-setter role, and a feeder keeps the rate fresh.
package app

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/evm"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// manualOracleABI is the minimal read ABI for ManualOracle.sol.
// getRate is order-sensitive: getRate(token0, token1) returns "token1 per token0"
// scaled to `decimals` fixed-point. Reverts Oracle__RateNotSet when unset.
const manualOracleABI = `[
{"type":"function","name":"getRate","stateMutability":"view","inputs":[
  {"name":"token0","type":"address"},
  {"name":"token1","type":"address"}
],"outputs":[
  {"name":"rate","type":"uint256"},
  {"name":"decimals","type":"uint8"}
]}
]`

// ManualOracleClient is a read-only EVM client for ManualOracle.sol.
type ManualOracleClient struct {
	contract common.Address
	ec       *ethclient.Client
	parsed   abi.ABI
	timeout  time.Duration
}

// ManualOracleConfig holds connection parameters for the adapter.
type ManualOracleConfig struct {
	RPCURL          string
	ContractAddress string
	Timeout         time.Duration
}

// NewManualOracleClient constructs a read-only ManualOracleClient.
func NewManualOracleClient(ctx context.Context, cfg ManualOracleConfig) (*ManualOracleClient, error) {
	if cfg.ContractAddress == "" {
		return nil, fmt.Errorf("ManualOracleConfig: ContractAddress is required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 15 * time.Second
	}
	ec, err := evm.Dial(ctx, cfg.RPCURL, cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("manual oracle: dial: %w", err)
	}
	parsed, err := evm.ParseABI(manualOracleABI)
	if err != nil {
		ec.Close()
		return nil, fmt.Errorf("manual oracle: parse ABI: %w", err)
	}
	return &ManualOracleClient{
		contract: common.HexToAddress(cfg.ContractAddress),
		ec:       ec,
		parsed:   parsed,
		timeout:  cfg.Timeout,
	}, nil
}

// Close releases the underlying RPC connection.
func (c *ManualOracleClient) Close() {
	if c.ec != nil {
		c.ec.Close()
	}
}

// GetRate returns the on-chain rate (token1 per token0) and its fixed-point decimals.
// The call reverts (returned as an error) when no rate has been set for the pair.
func (c *ManualOracleClient) GetRate(ctx context.Context, token0, token1 common.Address) (*big.Int, uint8, error) {
	input, err := c.parsed.Pack("getRate", token0, token1)
	if err != nil {
		return nil, 0, fmt.Errorf("manual oracle getRate pack: %w", err)
	}
	cctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	raw, err := c.ec.CallContract(cctx, ethereum.CallMsg{To: &c.contract, Data: input}, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("manual oracle getRate call: %w", err)
	}
	values, err := c.parsed.Methods["getRate"].Outputs.Unpack(raw)
	if err != nil {
		return nil, 0, fmt.Errorf("manual oracle getRate unpack: %w", err)
	}
	if len(values) != 2 {
		return nil, 0, fmt.Errorf("manual oracle getRate: expected 2 outputs, got %d", len(values))
	}
	rate, ok := values[0].(*big.Int)
	if !ok {
		return nil, 0, fmt.Errorf("manual oracle getRate: rate type %T", values[0])
	}
	decimals, ok := values[1].(uint8)
	if !ok {
		return nil, 0, fmt.Errorf("manual oracle getRate: decimals type %T", values[1])
	}
	return rate, decimals, nil
}

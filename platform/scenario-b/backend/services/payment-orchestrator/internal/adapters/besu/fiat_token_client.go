// SPDX-License-Identifier: Apache-2.0

// Package besu provides on-chain contract adapters for Besu.
package besu

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/evm"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

var _ ports.FiatTokenPort = (*FiatTokenClient)(nil)

// fiatTokenABIJSON is the minimal ABI for the FiatCentralBankMoney (fCeBM) ERC-20 contract.
// It includes decimals(), mint(address,uint256), burn(address,uint256), and balanceOf(address).
const fiatTokenABIJSON = `[{"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"symbol","outputs":[{"name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[{"name":"to","type":"address"},{"name":"amount","type":"uint256"}],"name":"mint","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"name":"from","type":"address"},{"name":"amount","type":"uint256"}],"name":"burn","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"name":"account","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]` // #nosec G101 -- not a secret; static ERC-20 contract ABI JSON

// FiatTokenClientConfig holds the configuration for the Besu FiatCentralBankMoney client.
type FiatTokenClientConfig struct {
	RPCURL           string // Besu JSON-RPC URL
	ChainID          int64  // Besu chain ID
	FiatTokenAddress string // Deployed FiatCentralBankMoney (fCeBM) address
	PrivateKeyHex    string // Operator private key for signing transactions (CENTRAL_BANK_ROLE)
}

// FiatTokenClient implements ports.FiatTokenPort using go-ethereum against the
// FiatCentralBankMoney (fCeBM) ERC-20 contract on Besu.
type FiatTokenClient struct {
	ethClient    *ethclient.Client
	tokenAddress common.Address
	tokenABI     abi.ABI
	// signer is the shared, nonce-serialized signer. It replaces a per-client transactor: this
	// client and its siblings share the operator key, and each fetching PendingNonceAt on its own
	// let two concurrent submissions claim the same nonce, with one silently replaced.
	signer *evm.Signer
	logger *slog.Logger
}

// NewFiatTokenClient creates a new Besu FiatCentralBankMoney client.
func NewFiatTokenClient(cfg FiatTokenClientConfig, logger *slog.Logger) (*FiatTokenClient, error) {
	ethClient, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("dial besu %s: %w", cfg.RPCURL, err)
	}

	parsed, err := abi.JSON(strings.NewReader(fiatTokenABIJSON))
	if err != nil {
		return nil, fmt.Errorf("parse fCeBM ABI: %w", err)
	}

	signer, err := evm.SharedSigner(cfg.PrivateKeyHex, big.NewInt(cfg.ChainID))
	if err != nil {
		return nil, fmt.Errorf("build signer: %w", err)
	}

	return &FiatTokenClient{
		ethClient:    ethClient,
		tokenAddress: common.HexToAddress(cfg.FiatTokenAddress),
		tokenABI:     parsed,
		signer:       signer,
		logger:       logger,
	}, nil
}

// Decimals returns the number of decimal places used by this token (e.g. 18 for standard ERC-20).
func (c *FiatTokenClient) Decimals(ctx context.Context) (uint8, error) {
	data, err := c.tokenABI.Pack("decimals")
	if err != nil {
		return 0, fmt.Errorf("pack decimals: %w", err)
	}
	result, err := c.ethClient.CallContract(ctx, ethereum.CallMsg{From: c.signer.Address(), To: &c.tokenAddress, Data: data}, nil)
	if err != nil {
		return 0, fmt.Errorf("call decimals: %w", err)
	}
	outputs, err := c.tokenABI.Unpack("decimals", result)
	if err != nil {
		return 0, fmt.Errorf("unpack decimals: %w", err)
	}
	if len(outputs) == 0 {
		return 0, fmt.Errorf("decimals: empty result")
	}
	decimals, ok := outputs[0].(uint8)
	if !ok {
		return 0, fmt.Errorf("decimals: unexpected type %T", outputs[0])
	}
	return decimals, nil
}

// Symbol returns the ERC-20 symbol of this token (e.g. "fCeBM_BRL"). The fiat currency
// code is the suffix after the underscore; callers may parse it for display.
func (c *FiatTokenClient) Symbol(ctx context.Context) (string, error) {
	data, err := c.tokenABI.Pack("symbol")
	if err != nil {
		return "", fmt.Errorf("pack symbol: %w", err)
	}
	result, err := c.ethClient.CallContract(ctx, ethereum.CallMsg{From: c.signer.Address(), To: &c.tokenAddress, Data: data}, nil)
	if err != nil {
		return "", fmt.Errorf("call symbol: %w", err)
	}
	outputs, err := c.tokenABI.Unpack("symbol", result)
	if err != nil {
		return "", fmt.Errorf("unpack symbol: %w", err)
	}
	if len(outputs) == 0 {
		return "", fmt.Errorf("symbol: empty result")
	}
	symbol, ok := outputs[0].(string)
	if !ok {
		return "", fmt.Errorf("symbol: unexpected type %T", outputs[0])
	}
	return symbol, nil
}

// Mint issues new fCeBM tokens to the specified Besu address.
func (c *FiatTokenClient) Mint(ctx context.Context, toAddress string, amount string) (string, error) {
	amountBig, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return "", fmt.Errorf("invalid amount: %s", amount)
	}

	data, err := c.tokenABI.Pack("mint", common.HexToAddress(toAddress), amountBig)
	if err != nil {
		return "", fmt.Errorf("pack mint: %w", err)
	}
	return c.sendFiatTx(ctx, data, "mint")
}

// Burn destroys fCeBM tokens from the specified Besu address.
func (c *FiatTokenClient) Burn(ctx context.Context, fromAddress string, amount string) (string, error) {
	amountBig, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return "", fmt.Errorf("invalid amount: %s", amount)
	}

	data, err := c.tokenABI.Pack("burn", common.HexToAddress(fromAddress), amountBig)
	if err != nil {
		return "", fmt.Errorf("pack burn: %w", err)
	}
	return c.sendFiatTx(ctx, data, "burn")
}

// BalanceOf returns the fCeBM balance of the given Besu address.
func (c *FiatTokenClient) BalanceOf(ctx context.Context, address string) (string, error) {
	data, err := c.tokenABI.Pack("balanceOf", common.HexToAddress(address))
	if err != nil {
		return "", fmt.Errorf("pack balanceOf: %w", err)
	}

	result, err := c.ethClient.CallContract(ctx, ethereum.CallMsg{From: c.signer.Address(), To: &c.tokenAddress, Data: data}, nil)
	if err != nil {
		return "", fmt.Errorf("call balanceOf: %w", err)
	}

	outputs, err := c.tokenABI.Unpack("balanceOf", result)
	if err != nil {
		return "", fmt.Errorf("unpack balanceOf: %w", err)
	}
	if len(outputs) == 0 {
		return "0", nil
	}
	balance, ok := outputs[0].(*big.Int)
	if !ok {
		return "0", nil
	}
	return balance.String(), nil
}

// GetFiatBalance returns the fCeBM balance for the configured operator address.
func (c *FiatTokenClient) GetFiatBalance(ctx context.Context) (string, error) {
	return c.BalanceOf(ctx, c.signer.Address().Hex())
}

// sendFiatTx signs and sends a transaction to the fCeBM contract, waiting for the receipt.
func (c *FiatTokenClient) sendFiatTx(ctx context.Context, data []byte, method string) (string, error) {
	// Submitted through the shared signer so every fCeBM transaction shares ONE nonce counter with
	// the rest of this process. This client used to build its own transactor and read
	// PendingNonceAt itself; with siblings on the same operator key, two concurrent submissions
	// could claim the same nonce and one would be silently replaced.
	_, txHash, err := evm.SubmitRawTxReceipt(ctx, c.ethClient, c.signer, c.tokenAddress, data, method)
	if err != nil {
		return "", err
	}
	c.logger.Info("fCeBM on-chain tx confirmed", "method", method, "tx_hash", txHash)
	return txHash, nil
}

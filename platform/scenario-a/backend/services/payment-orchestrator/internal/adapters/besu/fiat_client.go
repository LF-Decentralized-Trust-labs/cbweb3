// SPDX-License-Identifier: Apache-2.0

// Package besu provides on-chain contract adapters for Besu.
package besu

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log/slog"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

var _ ports.FiatTokenPort = (*FiatClient)(nil)

// fiatABIJSON is the minimal ABI for the FiatCentralBankMoney ERC-20 contract.
// It includes mint(address,uint256), burn(address,uint256), balanceOf(address),
// decimals() and symbol().
//
// decimals() and symbol() were added for ADR-009: the portal used to print the raw
// base-unit integer as if it were a currency amount, because nothing in Scenario A
// knew the token's scale. They are plain view calls on the standard ERC-20 surface.
const fiatABIJSON = `[{"inputs":[{"name":"to","type":"address"},{"name":"amount","type":"uint256"}],"name":"mint","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"name":"from","type":"address"},{"name":"amount","type":"uint256"}],"name":"burn","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"name":"account","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"symbol","outputs":[{"name":"","type":"string"}],"stateMutability":"view","type":"function"}]`

// FiatClientConfig holds the configuration for the Besu FiatCentralBankMoney client.
type FiatClientConfig struct {
	RPCURL           string // Besu JSON-RPC URL
	ChainID          int64  // Besu chain ID
	FiatTokenAddress string // Deployed FiatCentralBankMoney address
	PrivateKeyHex    string // Operator private key for signing transactions (CENTRAL_BANK_ROLE)
}

// FiatClient implements ports.FiatTokenPort using go-ethereum against the
// FiatCentralBankMoney (fCeBM) ERC-20 contract on Besu.
type FiatClient struct {
	ethClient   *ethclient.Client
	fiatAddress common.Address
	fiatABI     abi.ABI
	privateKey  *ecdsa.PrivateKey
	fromAddress common.Address
	chainID     *big.Int
	logger      *slog.Logger
}

// NewFiatClient creates a new Besu FiatCentralBankMoney client.
func NewFiatClient(cfg FiatClientConfig, logger *slog.Logger) (*FiatClient, error) {
	ethClient, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("dial besu %s: %w", cfg.RPCURL, err)
	}

	parsed, err := abi.JSON(strings.NewReader(fiatABIJSON))
	if err != nil {
		return nil, fmt.Errorf("parse fiat ABI: %w", err)
	}

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(cfg.PrivateKeyHex, "0x"))
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)

	return &FiatClient{
		ethClient:   ethClient,
		fiatAddress: common.HexToAddress(cfg.FiatTokenAddress),
		fiatABI:     parsed,
		privateKey:  privateKey,
		fromAddress: fromAddress,
		chainID:     big.NewInt(cfg.ChainID),
		logger:      logger,
	}, nil
}

// Mint issues new fCeBM tokens to the specified Besu address.
func (c *FiatClient) Mint(ctx context.Context, toAddress string, amount string) (string, error) {
	amountBig, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return "", fmt.Errorf("invalid amount: %s", amount)
	}

	data, err := c.fiatABI.Pack("mint", common.HexToAddress(toAddress), amountBig)
	if err != nil {
		return "", fmt.Errorf("pack mint: %w", err)
	}
	return c.sendTx(ctx, data, "mint")
}

// Burn destroys fCeBM tokens from the specified Besu address.
func (c *FiatClient) Burn(ctx context.Context, fromAddress string, amount string) (string, error) {
	amountBig, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return "", fmt.Errorf("invalid amount: %s", amount)
	}

	data, err := c.fiatABI.Pack("burn", common.HexToAddress(fromAddress), amountBig)
	if err != nil {
		return "", fmt.Errorf("pack burn: %w", err)
	}
	return c.sendTx(ctx, data, "burn")
}

// BalanceOf returns the fCeBM balance of the given Besu address.
func (c *FiatClient) BalanceOf(ctx context.Context, address string) (string, error) {
	data, err := c.fiatABI.Pack("balanceOf", common.HexToAddress(address))
	if err != nil {
		return "", fmt.Errorf("pack balanceOf: %w", err)
	}

	result, err := c.ethClient.CallContract(ctx, ethereum.CallMsg{From: c.fromAddress, To: &c.fiatAddress, Data: data}, nil)
	if err != nil {
		return "", fmt.Errorf("call balanceOf: %w", err)
	}

	outputs, err := c.fiatABI.Unpack("balanceOf", result)
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

// Decimals returns the token's decimal places, read from the contract.
//
// Not cached: the value is immutable for a deployed ERC-20, but the client is also
// used against freshly deployed tokens in tests and a stale cache there would be a
// confusing failure. One view call per balance read is not the cost worth optimising.
func (c *FiatClient) Decimals(ctx context.Context) (uint8, error) {
	data, err := c.fiatABI.Pack("decimals")
	if err != nil {
		return 0, fmt.Errorf("pack decimals: %w", err)
	}
	result, err := c.ethClient.CallContract(ctx, ethereum.CallMsg{From: c.fromAddress, To: &c.fiatAddress, Data: data}, nil)
	if err != nil {
		return 0, fmt.Errorf("call decimals: %w", err)
	}
	outputs, err := c.fiatABI.Unpack("decimals", result)
	if err != nil {
		return 0, fmt.Errorf("unpack decimals: %w", err)
	}
	if len(outputs) == 0 {
		return 0, fmt.Errorf("decimals returned no value")
	}
	d, ok := outputs[0].(uint8)
	if !ok {
		return 0, fmt.Errorf("decimals returned %T, want uint8", outputs[0])
	}
	return d, nil
}

// Symbol returns the token's symbol, read from the contract.
func (c *FiatClient) Symbol(ctx context.Context) (string, error) {
	data, err := c.fiatABI.Pack("symbol")
	if err != nil {
		return "", fmt.Errorf("pack symbol: %w", err)
	}
	result, err := c.ethClient.CallContract(ctx, ethereum.CallMsg{From: c.fromAddress, To: &c.fiatAddress, Data: data}, nil)
	if err != nil {
		return "", fmt.Errorf("call symbol: %w", err)
	}
	outputs, err := c.fiatABI.Unpack("symbol", result)
	if err != nil {
		return "", fmt.Errorf("unpack symbol: %w", err)
	}
	if len(outputs) == 0 {
		return "", fmt.Errorf("symbol returned no value")
	}
	sym, ok := outputs[0].(string)
	if !ok {
		return "", fmt.Errorf("symbol returned %T, want string", outputs[0])
	}
	return sym, nil
}

// GetFiatBalance returns the fCeBM balance for the configured operator address.
func (c *FiatClient) GetFiatBalance(ctx context.Context) (string, error) {
	return c.BalanceOf(ctx, c.fromAddress.Hex())
}

// sendTx signs and sends a transaction to the fCeBM contract, waiting for the receipt.
func (c *FiatClient) sendTx(ctx context.Context, data []byte, method string) (string, error) {
	nonce, err := c.ethClient.PendingNonceAt(ctx, c.fromAddress)
	if err != nil {
		return "", fmt.Errorf("get nonce: %w", err)
	}

	// Every spoke genesis sets zeroBaseFee and Besu runs with --min-gas-price=0
	// (see step_start_besu_found.go), so gas is always free by design. Do NOT use
	// SuggestGasPrice/eth_gasPrice here: Besu's gas price oracle can return a
	// non-zero default until enough zero-fee blocks accumulate after a spoke is
	// founded, which fails eth_estimateGas's upfront-cost check for any
	// zero-balance operator account (see client.go's sendTx for the full story).
	gasPrice := big.NewInt(0)

	auth, err := bind.NewKeyedTransactorWithChainID(c.privateKey, c.chainID)
	if err != nil {
		return "", fmt.Errorf("create transactor: %w", err)
	}
	auth.Nonce = new(big.Int).SetUint64(nonce)
	auth.GasPrice = gasPrice
	auth.GasLimit = 500_000
	auth.Context = ctx

	boundContract := bind.NewBoundContract(c.fiatAddress, c.fiatABI, c.ethClient, c.ethClient, c.ethClient)

	signedTx, err := boundContract.RawTransact(auth, data)
	if err != nil {
		return "", fmt.Errorf("send %s tx: %w", method, err)
	}

	receipt, err := bind.WaitMined(ctx, c.ethClient, signedTx)
	if err != nil {
		return "", fmt.Errorf("wait %s receipt: %w", method, err)
	}

	if receipt.Status == 0 {
		return "", fmt.Errorf("%s transaction reverted: %s", method, signedTx.Hash().Hex())
	}

	c.logger.Info("fiat token on-chain tx confirmed",
		"method", method,
		"tx_hash", signedTx.Hash().Hex(),
		"block", receipt.BlockNumber.Uint64(),
	)

	return signedTx.Hash().Hex(), nil
}

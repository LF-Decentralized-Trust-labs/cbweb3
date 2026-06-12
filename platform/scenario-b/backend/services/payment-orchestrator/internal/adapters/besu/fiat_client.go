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

var _ ports.TCeBMPort = (*TCeBMClient)(nil)

// tcebmABIJSON is the minimal ABI for the TokenizedCentralBankMoney (tCeBM) ERC-20 contract.
// It includes decimals(), mint(address,uint256), burn(address,uint256), and balanceOf(address).
const tcebmABIJSON = `[{"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"stateMutability":"view","type":"function"},{"inputs":[{"name":"to","type":"address"},{"name":"amount","type":"uint256"}],"name":"mint","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"name":"from","type":"address"},{"name":"amount","type":"uint256"}],"name":"burn","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"name":"account","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`

// TCeBMClientConfig holds the configuration for the Besu TokenizedCentralBankMoney client.
type TCeBMClientConfig struct {
	RPCURL        string // Besu JSON-RPC URL
	ChainID       int64  // Besu chain ID
	TokenAddress  string // Deployed TokenizedCentralBankMoney (tCeBM) address
	PrivateKeyHex string // Operator private key for signing transactions (CENTRAL_BANK_ROLE)
}

// TCeBMClient implements ports.TCeBMPort using go-ethereum against the
// TokenizedCentralBankMoney (tCeBM) ERC-20 contract on Besu.
type TCeBMClient struct {
	ethClient    *ethclient.Client
	tokenAddress common.Address
	tokenABI     abi.ABI
	privateKey   *ecdsa.PrivateKey
	fromAddress  common.Address
	chainID      *big.Int
	logger       *slog.Logger
}

// NewTCeBMClient creates a new Besu TokenizedCentralBankMoney client.
func NewTCeBMClient(cfg TCeBMClientConfig, logger *slog.Logger) (*TCeBMClient, error) {
	ethClient, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("dial besu %s: %w", cfg.RPCURL, err)
	}

	parsed, err := abi.JSON(strings.NewReader(tcebmABIJSON))
	if err != nil {
		return nil, fmt.Errorf("parse tCeBM ABI: %w", err)
	}

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(cfg.PrivateKeyHex, "0x"))
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)

	return &TCeBMClient{
		ethClient:    ethClient,
		tokenAddress: common.HexToAddress(cfg.TokenAddress),
		tokenABI:     parsed,
		privateKey:   privateKey,
		fromAddress:  fromAddress,
		chainID:      big.NewInt(cfg.ChainID),
		logger:       logger,
	}, nil
}

// Mint issues new tCeBM tokens to the specified Besu address.
func (c *TCeBMClient) Mint(ctx context.Context, toAddress string, amount string) (string, error) {
	amountBig, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return "", fmt.Errorf("invalid amount: %s", amount)
	}

	data, err := c.tokenABI.Pack("mint", common.HexToAddress(toAddress), amountBig)
	if err != nil {
		return "", fmt.Errorf("pack mint: %w", err)
	}
	return c.sendTx(ctx, data, "mint")
}

// Burn destroys tCeBM tokens from the specified Besu address.
func (c *TCeBMClient) Burn(ctx context.Context, fromAddress string, amount string) (string, error) {
	amountBig, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return "", fmt.Errorf("invalid amount: %s", amount)
	}

	data, err := c.tokenABI.Pack("burn", common.HexToAddress(fromAddress), amountBig)
	if err != nil {
		return "", fmt.Errorf("pack burn: %w", err)
	}
	return c.sendTx(ctx, data, "burn")
}

// BalanceOf returns the tCeBM balance of the given Besu address.
func (c *TCeBMClient) BalanceOf(ctx context.Context, address string) (string, error) {
	data, err := c.tokenABI.Pack("balanceOf", common.HexToAddress(address))
	if err != nil {
		return "", fmt.Errorf("pack balanceOf: %w", err)
	}

	result, err := c.ethClient.CallContract(ctx, ethereum.CallMsg{From: c.fromAddress, To: &c.tokenAddress, Data: data}, nil)
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

// GetBalance returns the tCeBM balance for the configured operator address.
func (c *TCeBMClient) GetBalance(ctx context.Context) (string, error) {
	return c.BalanceOf(ctx, c.fromAddress.Hex())
}

// Decimals returns the number of decimal places used by this token (e.g. 18 for standard ERC-20).
func (c *TCeBMClient) Decimals(ctx context.Context) (uint8, error) {
	data, err := c.tokenABI.Pack("decimals")
	if err != nil {
		return 0, fmt.Errorf("pack decimals: %w", err)
	}
	result, err := c.ethClient.CallContract(ctx, ethereum.CallMsg{From: c.fromAddress, To: &c.tokenAddress, Data: data}, nil)
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

// sendTx signs and sends a transaction to the tCeBM contract, waiting for the receipt.
func (c *TCeBMClient) sendTx(ctx context.Context, data []byte, method string) (string, error) {
	nonce, err := c.ethClient.PendingNonceAt(ctx, c.fromAddress)
	if err != nil {
		return "", fmt.Errorf("get nonce: %w", err)
	}

	gasPrice, err := c.ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("suggest gas price: %w", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(c.privateKey, c.chainID)
	if err != nil {
		return "", fmt.Errorf("create transactor: %w", err)
	}
	auth.Nonce = new(big.Int).SetUint64(nonce)
	auth.GasPrice = gasPrice
	auth.GasLimit = 500_000
	auth.Context = ctx

	boundContract := bind.NewBoundContract(c.tokenAddress, c.tokenABI, c.ethClient, c.ethClient, c.ethClient)

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

	c.logger.Info("tCeBM on-chain tx confirmed",
		"method", method,
		"tx_hash", signedTx.Hash().Hex(),
		"block", receipt.BlockNumber.Uint64(),
	)

	return signedTx.Hash().Hex(), nil
}

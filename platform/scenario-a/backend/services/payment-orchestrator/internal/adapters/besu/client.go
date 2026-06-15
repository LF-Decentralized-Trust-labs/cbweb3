// SPDX-License-Identifier: Apache-2.0

// Package besu provides the on-chain HTLC contract adapter for Besu.
// It calls HashTimeLockedContract.sol's lock/settle/refund via go-ethereum.
package besu

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

var _ ports.HTLCContractPort = (*Client)(nil)

const htlcABIJSON = `[{"inputs":[{"name":"contractId","type":"bytes32"},{"name":"receiver","type":"address"},{"name":"hashLock","type":"bytes32"},{"name":"timeLock","type":"uint256"},{"name":"zetoLockRef","type":"bytes32"},{"name":"agreementId","type":"bytes32"}],"name":"lock","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"name":"contractId","type":"bytes32"},{"name":"secret","type":"bytes32"}],"name":"settle","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"name":"contractId","type":"bytes32"}],"name":"refund","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"name":"commitment","type":"bytes32"}],"name":"registerAgreementCommitment","outputs":[],"stateMutability":"nonpayable","type":"function"}]`

// ClientConfig holds the configuration for the Besu HTLC client.
type ClientConfig struct {
	RPCURL        string // Besu JSON-RPC URL
	ChainID       int64  // Besu chain ID
	HTLCAddress   string // Deployed HashTimeLockedContract address
	PrivateKeyHex string // Operator private key for signing transactions
}

// Client implements ports.HTLCContractPort using go-ethereum.
type Client struct {
	ethClient   *ethclient.Client
	htlcAddress common.Address
	htlcABI     abi.ABI
	privateKey  *ecdsa.PrivateKey
	fromAddress common.Address
	chainID     *big.Int
	logger      *slog.Logger
}

// NewClient creates a new Besu HTLC client.
func NewClient(cfg ClientConfig, logger *slog.Logger) (*Client, error) {
	ethClient, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("dial besu %s: %w", cfg.RPCURL, err)
	}

	parsed, err := abi.JSON(strings.NewReader(htlcABIJSON))
	if err != nil {
		return nil, fmt.Errorf("parse HTLC ABI: %w", err)
	}

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(cfg.PrivateKeyHex, "0x"))
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)

	return &Client{
		ethClient:   ethClient,
		htlcAddress: common.HexToAddress(cfg.HTLCAddress),
		htlcABI:     parsed,
		privateKey:  privateKey,
		fromAddress: fromAddress,
		chainID:     big.NewInt(cfg.ChainID),
		logger:      logger,
	}, nil
}

func (c *Client) Lock(ctx context.Context, params ports.HTLCLockParams) (string, error) {
	// The receiver in the gRPC request is a Paladin identity (e.g. "funded_operator@spoke-a-bank-c"),
	// not an Ethereum address. Fall back to the operator's own address for the on-chain coordination
	// record, since the actual token recipient is tracked by Zeto/Paladin.
	receiver := common.HexToAddress(params.Receiver)
	if receiver == (common.Address{}) {
		receiver = c.fromAddress
	}
	data, err := c.htlcABI.Pack("lock",
		params.ContractID,
		receiver,
		params.HashLock,
		new(big.Int).SetUint64(params.TimeLock),
		params.ZetoLockRef,
		params.AgreementID,
	)
	if err != nil {
		return "", fmt.Errorf("pack lock: %w", err)
	}
	return c.sendTx(ctx, data, "lock")
}

func (c *Client) Settle(ctx context.Context, contractID [32]byte, secret [32]byte) (string, error) {
	data, err := c.htlcABI.Pack("settle", contractID, secret)
	if err != nil {
		return "", fmt.Errorf("pack settle: %w", err)
	}
	return c.sendTx(ctx, data, "settle")
}

func (c *Client) Refund(ctx context.Context, contractID [32]byte) (string, error) {
	data, err := c.htlcABI.Pack("refund", contractID)
	if err != nil {
		return "", fmt.Errorf("pack refund: %w", err)
	}
	return c.sendTx(ctx, data, "refund")
}

func (c *Client) RegisterAgreementCommitment(ctx context.Context, commitment [32]byte) (string, error) {
	data, err := c.htlcABI.Pack("registerAgreementCommitment", commitment)
	if err != nil {
		return "", fmt.Errorf("pack registerAgreementCommitment: %w", err)
	}
	return c.sendTx(ctx, data, "registerAgreementCommitment")
}

func (c *Client) sendTx(ctx context.Context, data []byte, method string) (string, error) {
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

	// Estimate gas first to detect contract reverts before submitting the tx.
	msg := ethereum.CallMsg{
		From:     c.fromAddress,
		To:       &c.htlcAddress,
		GasPrice: gasPrice,
		Data:     data,
	}
	if estimatedGas, estErr := c.ethClient.EstimateGas(ctx, msg); estErr != nil {
		return "", fmt.Errorf("%s call would revert: %w", method, estErr)
	} else {
		auth.GasLimit = estimatedGas * 120 / 100 // 20% headroom
	}

	boundContract := bind.NewBoundContract(c.htlcAddress, c.htlcABI, c.ethClient, c.ethClient, c.ethClient)

	signedTx, err := boundContract.RawTransact(auth, data)
	if err != nil {
		return "", fmt.Errorf("send %s tx: %w", method, err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	receipt, err := bind.WaitMined(waitCtx, c.ethClient, signedTx)
	if err != nil {
		return "", fmt.Errorf("wait %s receipt: %w", method, err)
	}

	if receipt.Status == 0 {
		return "", fmt.Errorf("%s transaction reverted: %s", method, signedTx.Hash().Hex())
	}

	c.logger.Info("HTLC on-chain tx confirmed",
		"method", method,
		"tx_hash", signedTx.Hash().Hex(),
		"block", receipt.BlockNumber.Uint64(),
	)

	return signedTx.Hash().Hex(), nil
}

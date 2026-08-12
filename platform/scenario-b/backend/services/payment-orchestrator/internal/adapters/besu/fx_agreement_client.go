// SPDX-License-Identifier: Apache-2.0

// Package besu provides on-chain contract adapters for Besu.
package besu

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/evm"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

var _ ports.FXAgreementContractPort = (*FXAgreementClient)(nil)

const fxAgreementABIJSON = `[` +
	`{"inputs":[{"name":"tradeId","type":"bytes32"},{"name":"counterpartyB","type":"address"},{"name":"settlementAgent","type":"address"},{"name":"custodian","type":"address"},{"name":"beneficiary","type":"address"},{"name":"originAmount","type":"uint256"},{"name":"counterAmount","type":"uint256"},{"name":"originCurrency","type":"bytes32"},{"name":"counterCurrency","type":"bytes32"},{"name":"rate","type":"uint256"},{"name":"expiryDate","type":"uint256"}],"name":"propose","outputs":[],"stateMutability":"nonpayable","type":"function"},` +
	`{"inputs":[{"name":"tradeId","type":"bytes32"},{"name":"originator","type":"address"},{"name":"counterpartyB","type":"address"},{"name":"settlementAgent","type":"address"},{"name":"custodian","type":"address"},{"name":"beneficiary","type":"address"},{"name":"originAmount","type":"uint256"},{"name":"counterAmount","type":"uint256"},{"name":"originCurrency","type":"bytes32"},{"name":"counterCurrency","type":"bytes32"},{"name":"rate","type":"uint256"},{"name":"expiryDate","type":"uint256"}],"name":"proposeOnBehalf","outputs":[],"stateMutability":"nonpayable","type":"function"},` +
	`{"inputs":[{"name":"tradeId","type":"bytes32"}],"name":"accept","outputs":[],"stateMutability":"nonpayable","type":"function"},` +
	`{"inputs":[{"name":"tradeId","type":"bytes32"}],"name":"acceptOnBehalf","outputs":[],"stateMutability":"nonpayable","type":"function"},` +
	`{"inputs":[{"name":"tradeId","type":"bytes32"}],"name":"reject","outputs":[],"stateMutability":"nonpayable","type":"function"},` +
	`{"inputs":[{"name":"tradeId","type":"bytes32"}],"name":"rejectOnBehalf","outputs":[],"stateMutability":"nonpayable","type":"function"},` +
	`{"inputs":[{"name":"tradeId","type":"bytes32"}],"name":"cancel","outputs":[],"stateMutability":"nonpayable","type":"function"},` +
	`{"inputs":[{"name":"tradeId","type":"bytes32"}],"name":"settle","outputs":[],"stateMutability":"nonpayable","type":"function"}` +
	`]`

// FXAgreementClientConfig holds the configuration for the Besu FXAgreement client.
type FXAgreementClientConfig struct {
	RPCURL             string // Besu JSON-RPC URL
	ChainID            int64  // Besu chain ID
	FXAgreementAddress string // Deployed FXAgreement address
	PrivateKeyHex      string // Operator private key for signing transactions
}

// FXAgreementClient implements ports.FXAgreementContractPort using go-ethereum.
type FXAgreementClient struct {
	ethClient *ethclient.Client
	fxAddress common.Address
	fxABI     abi.ABI
	// signer is the shared, nonce-serialized signer (see sendTx).
	signer *evm.Signer
	logger *slog.Logger
}

// NewFXAgreementClient creates a new Besu FXAgreement client.
func NewFXAgreementClient(cfg FXAgreementClientConfig, logger *slog.Logger) (*FXAgreementClient, error) {
	ethClient, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("dial besu %s: %w", cfg.RPCURL, err)
	}

	parsed, err := abi.JSON(strings.NewReader(fxAgreementABIJSON))
	if err != nil {
		return nil, fmt.Errorf("parse FXAgreement ABI: %w", err)
	}

	signer, err := evm.SharedSigner(cfg.PrivateKeyHex, big.NewInt(cfg.ChainID))
	if err != nil {
		return nil, fmt.Errorf("build signer: %w", err)
	}

	return &FXAgreementClient{
		ethClient: ethClient,
		fxAddress: common.HexToAddress(cfg.FXAgreementAddress),
		fxABI:     parsed,
		signer:    signer,
		logger:    logger,
	}, nil
}

func (c *FXAgreementClient) Propose(ctx context.Context, params ports.FXProposalParams) (string, error) {
	data, err := c.fxABI.Pack("propose",
		params.TradeID,
		params.CounterpartyB,
		params.SettlementAgent,
		params.Custodian,
		params.Beneficiary,
		params.OriginAmount,
		params.CounterAmount,
		params.OriginCurrency,
		params.CounterCurrency,
		params.Rate,
		params.ExpiryDate,
	)
	if err != nil {
		return "", fmt.Errorf("pack propose: %w", err)
	}
	return c.sendTx(ctx, data, "propose")
}

func (c *FXAgreementClient) ProposeOnBehalf(ctx context.Context, params ports.FXProposalParams) (string, error) {
	data, err := c.fxABI.Pack("proposeOnBehalf",
		params.TradeID,
		params.Originator,
		params.CounterpartyB,
		params.SettlementAgent,
		params.Custodian,
		params.Beneficiary,
		params.OriginAmount,
		params.CounterAmount,
		params.OriginCurrency,
		params.CounterCurrency,
		params.Rate,
		params.ExpiryDate,
	)
	if err != nil {
		return "", fmt.Errorf("pack proposeOnBehalf: %w", err)
	}
	return c.sendTx(ctx, data, "proposeOnBehalf")
}

func (c *FXAgreementClient) Accept(ctx context.Context, tradeID [32]byte) (string, error) {
	data, err := c.fxABI.Pack("accept", tradeID)
	if err != nil {
		return "", fmt.Errorf("pack accept: %w", err)
	}
	return c.sendTx(ctx, data, "accept")
}

func (c *FXAgreementClient) AcceptOnBehalf(ctx context.Context, tradeID [32]byte) (string, error) {
	data, err := c.fxABI.Pack("acceptOnBehalf", tradeID)
	if err != nil {
		return "", fmt.Errorf("pack acceptOnBehalf: %w", err)
	}
	return c.sendTx(ctx, data, "acceptOnBehalf")
}

func (c *FXAgreementClient) Reject(ctx context.Context, tradeID [32]byte) (string, error) {
	data, err := c.fxABI.Pack("reject", tradeID)
	if err != nil {
		return "", fmt.Errorf("pack reject: %w", err)
	}
	return c.sendTx(ctx, data, "reject")
}

func (c *FXAgreementClient) RejectOnBehalf(ctx context.Context, tradeID [32]byte) (string, error) {
	data, err := c.fxABI.Pack("rejectOnBehalf", tradeID)
	if err != nil {
		return "", fmt.Errorf("pack rejectOnBehalf: %w", err)
	}
	return c.sendTx(ctx, data, "rejectOnBehalf")
}

func (c *FXAgreementClient) Cancel(ctx context.Context, tradeID [32]byte) (string, error) {
	data, err := c.fxABI.Pack("cancel", tradeID)
	if err != nil {
		return "", fmt.Errorf("pack cancel: %w", err)
	}
	return c.sendTx(ctx, data, "cancel")
}

func (c *FXAgreementClient) Settle(ctx context.Context, tradeID [32]byte) (string, error) {
	data, err := c.fxABI.Pack("settle", tradeID)
	if err != nil {
		return "", fmt.Errorf("pack settle: %w", err)
	}
	return c.sendTx(ctx, data, "settle")
}

func (c *FXAgreementClient) sendTx(ctx context.Context, data []byte, method string) (string, error) {
	// Pre-flight estimate, kept as this client's own policy: a revert detected here fails the call
	// BEFORE a transaction is submitted, which the shared submission path does not do (it falls back
	// to a conservative gas limit and lets the revert surface in the receipt). For FX agreements the
	// earlier signal is worth the extra call, so it stays.
	gasPrice, err := c.ethClient.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("suggest gas price: %w", err)
	}
	msg := ethereum.CallMsg{
		From:     c.signer.Address(),
		To:       &c.fxAddress,
		GasPrice: gasPrice,
		Data:     data,
	}
	if _, estErr := c.ethClient.EstimateGas(ctx, msg); estErr != nil {
		return "", fmt.Errorf("%s call would revert: %w", method, estErr)
	}

	// Submitted through the shared signer so every FX transaction shares ONE nonce counter with the
	// sibling clients on the same operator key. Each of them used to read PendingNonceAt on its own,
	// which let two concurrent submissions claim the same nonce and one be silently replaced.
	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	_, txHash, err := evm.SubmitRawTxReceipt(waitCtx, c.ethClient, c.signer, c.fxAddress, data, method)
	if err != nil {
		return "", err
	}

	c.logger.Info("FXAgreement on-chain tx confirmed",
		"method", method,
		"tx_hash", txHash,
	)
	return txHash, nil
}

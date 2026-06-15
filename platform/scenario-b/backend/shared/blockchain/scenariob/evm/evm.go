// SPDX-License-Identifier: Apache-2.0

// Package evm provides shared helpers for building Scenario B EVM clients on top of
// go-ethereum/ethclient. Each concrete client (AMM, SpokeBridge) reuses these helpers to
// dial the Hub or Spoke RPC, sign transactions from a hex-encoded private key, and bind
// Solidity ABIs parsed at construction time.
package evm

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Signer carries the private key and chain ID needed to sign and submit transactions.
type Signer struct {
	key     *ecdsa.PrivateKey
	address common.Address
	chainID *big.Int
}

// NewSigner parses a hex-encoded secp256k1 private key and binds it to a chain ID.
// The hex string may start with "0x".
func NewSigner(hexKey string, chainID *big.Int) (*Signer, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(hexKey), "0x")
	if trimmed == "" {
		return nil, errors.New("empty private key")
	}
	if chainID == nil || chainID.Sign() <= 0 {
		return nil, errors.New("chainID must be a positive integer")
	}
	key, err := crypto.HexToECDSA(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}
	return &Signer{
		key:     key,
		address: crypto.PubkeyToAddress(key.PublicKey),
		chainID: new(big.Int).Set(chainID),
	}, nil
}

// Address returns the signer's derived address.
func (s *Signer) Address() common.Address { return s.address }

// TransactOpts builds a TransactOpts instance for bind.BoundContract calls.
// The returned opts has its Context set; the caller may override gas limits after.
func (s *Signer) TransactOpts(ctx context.Context) (*bind.TransactOpts, error) {
	opts, err := bind.NewKeyedTransactorWithChainID(s.key, s.chainID)
	if err != nil {
		return nil, fmt.Errorf("build transact opts: %w", err)
	}
	opts.Context = ctx
	return opts, nil
}

// ChainID returns the configured chain ID.
func (s *Signer) ChainID() *big.Int { return new(big.Int).Set(s.chainID) }

// ParseABI trims whitespace and parses a JSON ABI string. Returned value is safe to share
// across goroutines.
func ParseABI(raw string) (abi.ABI, error) {
	return abi.JSON(strings.NewReader(raw))
}

// Dial establishes a connection to an EVM JSON-RPC endpoint with a short timeout.
func Dial(ctx context.Context, rpcURL string, timeout time.Duration) (*ethclient.Client, error) {
	if rpcURL == "" {
		return nil, errors.New("rpcURL is required")
	}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client, err := ethclient.DialContext(dialCtx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", rpcURL, err)
	}
	return client, nil
}

// Call executes a read-only eth_call against the given contract using the parsed ABI method.
// Outputs are unpacked into `results` (a slice of pointers).
func Call(
	ctx context.Context,
	ec *ethclient.Client,
	contract common.Address,
	parsedABI abi.ABI,
	method string,
	args []interface{},
	results ...interface{},
) error {
	input, err := parsedABI.Pack(method, args...)
	if err != nil {
		return fmt.Errorf("pack %s: %w", method, err)
	}
	msg := ethereum.CallMsg{To: &contract, Data: input}
	raw, err := ec.CallContract(ctx, msg, nil)
	if err != nil {
		return fmt.Errorf("call %s: %w", method, err)
	}
	if len(raw) == 0 {
		return fmt.Errorf("call %s: empty response", method)
	}
	unpacked, err := parsedABI.Unpack(method, raw)
	if err != nil {
		return fmt.Errorf("unpack %s: %w", method, err)
	}
	if len(unpacked) != len(results) {
		return fmt.Errorf("%s: expected %d outputs, got %d", method, len(results), len(unpacked))
	}
	for i, out := range unpacked {
		if err := copyOutput(out, results[i]); err != nil {
			return fmt.Errorf("%s output %d: %w", method, i, err)
		}
	}
	return nil
}

// SubmitTx signs and submits a transaction invoking `method` on `contract`; it waits for
// the receipt (up to the ctx deadline) and returns the confirmed transaction hash.
func SubmitTx(
	ctx context.Context,
	ec *ethclient.Client,
	signer *Signer,
	contract common.Address,
	parsedABI abi.ABI,
	method string,
	args ...interface{},
) (string, error) {
	_, txHash, err := submitTxInternal(ctx, ec, signer, contract, parsedABI, method, args...)
	return txHash, err
}

// SubmitTxReceipt is like SubmitTx but also returns the mined receipt so callers can
// parse events from the transaction logs.
func SubmitTxReceipt(
	ctx context.Context,
	ec *ethclient.Client,
	signer *Signer,
	contract common.Address,
	parsedABI abi.ABI,
	method string,
	args ...interface{},
) (*types.Receipt, string, error) {
	return submitTxInternal(ctx, ec, signer, contract, parsedABI, method, args...)
}

func submitTxInternal(
	ctx context.Context,
	ec *ethclient.Client,
	signer *Signer,
	contract common.Address,
	parsedABI abi.ABI,
	method string,
	args ...interface{},
) (*types.Receipt, string, error) {
	input, err := parsedABI.Pack(method, args...)
	if err != nil {
		return nil, "", fmt.Errorf("pack %s: %w", method, err)
	}
	nonce, err := ec.PendingNonceAt(ctx, signer.Address())
	if err != nil {
		return nil, "", fmt.Errorf("nonce: %w", err)
	}
	gasPrice, err := ec.SuggestGasPrice(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("gas price: %w", err)
	}
	msg := ethereum.CallMsg{
		From:     signer.Address(),
		To:       &contract,
		Data:     input,
		GasPrice: gasPrice,
	}
	gasLimit, err := ec.EstimateGas(ctx, msg)
	if err != nil {
		gasLimit = 500_000 // conservative fallback for Besu dev networks
	}
	tx := types.NewTransaction(nonce, contract, big.NewInt(0), gasLimit, gasPrice, input)
	signed, err := types.SignTx(tx, types.NewLondonSigner(signer.ChainID()), signer.key)
	if err != nil {
		return nil, "", fmt.Errorf("sign tx: %w", err)
	}
	if err := ec.SendTransaction(ctx, signed); err != nil {
		return nil, "", fmt.Errorf("send tx: %w", err)
	}
	receipt, err := bind.WaitMined(ctx, ec, signed)
	if err != nil {
		return nil, "", fmt.Errorf("wait mined: %w", err)
	}
	if receipt.Status == 0 {
		return nil, "", fmt.Errorf("transaction reverted on-chain (tx=%s) — check contract permissions and token allowances", signed.Hash().Hex())
	}
	return receipt, signed.Hash().Hex(), nil
}

// copyOutput copies src into dst through reflection; dst must be a pointer.
func copyOutput(src, dst interface{}) error {
	switch v := dst.(type) {
	case *big.Int:
		bi, ok := src.(*big.Int)
		if !ok {
			return fmt.Errorf("expected *big.Int, got %T", src)
		}
		v.Set(bi)
	case *bool:
		b, ok := src.(bool)
		if !ok {
			return fmt.Errorf("expected bool, got %T", src)
		}
		*v = b
	case *common.Address:
		a, ok := src.(common.Address)
		if !ok {
			return fmt.Errorf("expected common.Address, got %T", src)
		}
		*v = a
	case *[32]byte:
		b, ok := src.([32]byte)
		if !ok {
			return fmt.Errorf("expected [32]byte, got %T", src)
		}
		*v = b
	case *string:
		s, ok := src.(string)
		if !ok {
			return fmt.Errorf("expected string, got %T", src)
		}
		*v = s
	default:
		return fmt.Errorf("unsupported output type %T", dst)
	}
	return nil
}

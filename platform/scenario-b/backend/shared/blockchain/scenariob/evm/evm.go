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
	"sync"
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
// mu serializes nonce assignment through SendTransaction so concurrent callers cannot
// collide on the same nonce — see signAndSend.
type Signer struct {
	// key is set only for a LOCAL signer. A remote one holds no key material — that is the point of
	// production custody, where the key never leaves the provider.
	key *ecdsa.PrivateKey
	// remote and keyID are set only for a remote signer; see NewRemoteSigner.
	remote    KeySigner
	keyID     string
	address   common.Address
	chainID   *big.Int
	mu        sync.Mutex
	nonce     uint64
	nonceInit bool
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

// sharedSigners memoizes one Signer per (account, chain) for the lifetime of the process.
//
// A Signer owns a nonce counter, so a SECOND Signer built from the same key is a second counter on
// ONE account, and the two drift apart the moment either of them sends. When they then submit
// concurrently they claim the same nonce; on a zero-gas chain the price-bump check that normally
// rejects a replacement passes (0 is not less than 0), so the later transaction replaces the earlier
// one in the mempool. The replaced hash is never mined and its caller waits for a receipt that will
// never exist.
//
// This is not hypothetical. On the sample stack a fCeBM mint and the bridge relayer's residue leg —
// both signing with the central bank's spoke operator key, both inside the payment-orchestrator
// process, each holding its own Signer — collided on nonce 15: the relayer's transaction mined and
// the mint vanished, hanging the deposit-approval request indefinitely.
//
// Keying by derived address rather than by the hex string keeps the "0x"-prefixed and bare forms of
// the same key on one counter.
var (
	sharedSignersMu sync.Mutex
	sharedSigners   = map[string]*Signer{}
)

// SharedSigner returns the process-wide Signer for (hexKey, chainID), building it on first use.
//
// Production wiring must use this rather than NewSigner: several clients are routinely handed the
// same operator key, and only a shared instance gives them the one nonce counter that
// signAndSend's mutex can actually serialize. NewSigner stays available for tests and for callers
// that deliberately want an isolated counter.
func SharedSigner(hexKey string, chainID *big.Int) (*Signer, error) {
	candidate, err := NewSigner(hexKey, chainID)
	if err != nil {
		return nil, err
	}
	registryKey := candidate.address.Hex() + "@" + chainID.String()

	sharedSignersMu.Lock()
	defer sharedSignersMu.Unlock()
	if existing, ok := sharedSigners[registryKey]; ok {
		return existing, nil
	}
	sharedSigners[registryKey] = candidate
	return candidate, nil
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
	_, txHash, err := submitTxInternal(ctx, ec, signer, contract, parsedABI, method, nil, args...)
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
	return submitTxInternal(ctx, ec, signer, contract, parsedABI, method, nil, args...)
}

// SubmitTxAwaitBroadcast is like SubmitTx but invokes onBroadcast with the transaction hash
// as soon as the node accepts the transaction into its mempool — before waiting for the
// receipt. This lets callers persist a pre-confirmation intent record so a process crash or
// a WaitMined timeout on an already-broadcast transaction can be reconciled by hash on retry
// (via TxMined) instead of blindly re-submitting a non-idempotent operation such as a burn or
// mint. onBroadcast is a best-effort notification (it must handle and log its own errors); it
// never aborts the wait, since the transaction is already in flight and the caller's
// authoritative post-confirmation write is the source of truth. onBroadcast may be nil.
func SubmitTxAwaitBroadcast(
	ctx context.Context,
	ec *ethclient.Client,
	signer *Signer,
	contract common.Address,
	parsedABI abi.ABI,
	method string,
	onBroadcast func(txHash string),
	args ...interface{},
) (string, error) {
	_, txHash, err := submitTxInternal(ctx, ec, signer, contract, parsedABI, method, onBroadcast, args...)
	return txHash, err
}

// TxMined reports whether the transaction identified by txHash has a receipt yet, and if so
// whether it succeeded (receipt.Status == 1). A not-yet-mined transaction returns
// (false, false, nil) so callers can distinguish "still pending / dropped" from "mined and
// reverted" and fail closed accordingly. It never re-broadcasts.
func TxMined(ctx context.Context, ec *ethclient.Client, txHash string) (mined bool, success bool, err error) {
	receipt, rerr := ec.TransactionReceipt(ctx, common.HexToHash(txHash))
	if rerr != nil {
		if errors.Is(rerr, ethereum.NotFound) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("get receipt %s: %w", txHash, rerr)
	}
	return true, receipt.Status == 1, nil
}

func submitTxInternal(
	ctx context.Context,
	ec *ethclient.Client,
	signer *Signer,
	contract common.Address,
	parsedABI abi.ABI,
	method string,
	onBroadcast func(txHash string),
	args ...interface{},
) (*types.Receipt, string, error) {
	input, err := parsedABI.Pack(method, args...)
	if err != nil {
		return nil, "", fmt.Errorf("pack %s: %w", method, err)
	}
	return submitRawInternal(ctx, ec, signer, contract, input, method, onBroadcast)
}

// SubmitRawTxReceipt submits PRE-PACKED calldata through this package's serialized nonce counter
// and returns the receipt.
//
// It exists for callers that pack their own calldata and previously built their own transactor —
// three payment-orchestrator clients did, each fetching PendingNonceAt independently while sharing
// the operator key. Same key, same chain, no shared counter: two concurrent submissions could claim
// the same nonce and one would be replaced. Routing them here puts every submission behind the one
// counter in signAndSend.
//
// label names the call in error messages only; it does not affect encoding.
func SubmitRawTxReceipt(
	ctx context.Context,
	ec *ethclient.Client,
	signer *Signer,
	contract common.Address,
	input []byte,
	label string,
) (*types.Receipt, string, error) {
	if signer == nil {
		return nil, "", errors.New("evm: signer is required to submit a transaction")
	}
	if len(input) == 0 {
		// A transaction with no calldata is a plain value transfer, not the contract call the
		// caller meant to make. Refuse rather than send it.
		return nil, "", fmt.Errorf("evm: %s: empty calldata", label)
	}
	return submitRawInternal(ctx, ec, signer, contract, input, label, nil)
}

func submitRawInternal(
	ctx context.Context,
	ec *ethclient.Client,
	signer *Signer,
	contract common.Address,
	input []byte,
	label string,
	onBroadcast func(txHash string),
) (*types.Receipt, string, error) {
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
	signed, err := signer.signAndSend(ctx, ec, contract, gasLimit, gasPrice, input)
	if err != nil {
		return nil, "", err
	}
	// The transaction is now in the mempool. Notify the caller with the hash (best-effort,
	// before the multi-second WaitMined) so it can record a pre-confirmation intent that makes
	// a crash or timeout reconcilable by hash on retry instead of a blind re-submission.
	if onBroadcast != nil {
		onBroadcast(signed.Hash().Hex())
	}
	receipt, err := WaitForReceipt(ctx, ec, signed, label)
	if err != nil {
		return nil, "", err
	}
	return receipt, signed.Hash().Hex(), nil
}

// WaitForReceipt waits for tx to be mined and refuses a reverted receipt.
//
// It is exported for the one submission shape this package cannot own: a contract DEPLOY, which
// go-ethereum's generated bindings broadcast themselves and hand back as a transaction. Those
// callers still need the same deadline policy as everything else — a deploy that loses a nonce race
// hangs exactly like a mint does.
func WaitForReceipt(ctx context.Context, ec *ethclient.Client, tx *types.Transaction, label string) (*types.Receipt, error) {
	waitCtx, cancel := receiptWaitContext(ctx)
	defer cancel()
	receipt, err := bind.WaitMined(waitCtx, ec, tx)
	if err != nil {
		// The hash is part of the error on purpose: a submission that is not mined within the
		// deadline may still be in the mempool, and TxMined on this hash is what tells a retry
		// apart from a re-submission of a non-idempotent burn or mint.
		return nil, fmt.Errorf("wait mined (%s tx=%s): %w", label, tx.Hash().Hex(), err)
	}
	if receipt.Status == 0 {
		return nil, fmt.Errorf("%s: transaction reverted on-chain (tx=%s) — check contract permissions and token allowances", label, tx.Hash().Hex())
	}
	return receipt, nil
}

// ReceiptWaitTimeout bounds the wait for a receipt when the caller's context carries no deadline of
// its own. A caller that sets one keeps it — this only closes the case where nothing bounds the wait.
//
// Without it, a transaction that is dropped from the mempool (a nonce collision with another sender
// on the same account is the way that happens here) parks the caller forever: WaitMined polls for a
// receipt that will never be written, no error is ever logged, and the request never returns. A
// deadline turns that into a reportable, retryable failure. It is generous relative to QBFT block
// time (2–4 s locally) so it never fires on a merely busy chain.
var ReceiptWaitTimeout = 90 * time.Second

// receiptWaitContext derives the context WaitMined runs under. Exposed as a helper so the deadline
// policy — inherit the caller's, otherwise impose ReceiptWaitTimeout — is testable on its own.
func receiptWaitContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok || ReceiptWaitTimeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, ReceiptWaitTimeout)
}

// signAndSend assigns a unique nonce, signs, and broadcasts the transaction while holding
// signer.mu. The lock is released as soon as the tx is in the node's mempool so that
// bind.WaitMined (which can take 2–4 s on QBFT) runs outside the critical section and
// concurrent callers can queue their own transactions without blocking on mining.
//
// On "nonce too low" (e.g. after a node restart or manual tx), the counter is re-synced
// from PendingNonceAt and the submission is retried once.
func (s *Signer) signAndSend(
	ctx context.Context,
	ec *ethclient.Client,
	contract common.Address,
	gasLimit uint64,
	gasPrice *big.Int,
	input []byte,
) (*types.Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.nonceInit {
		n, err := ec.PendingNonceAt(ctx, s.Address())
		if err != nil {
			return nil, fmt.Errorf("nonce: %w", err)
		}
		s.nonce = n
		s.nonceInit = true
	}

	for attempt := 0; attempt < 2; attempt++ {
		tx := types.NewTransaction(s.nonce, contract, big.NewInt(0), gasLimit, gasPrice, input)
		signed, err := s.signTx(ctx, tx)
		if err != nil {
			return nil, err
		}
		if err = ec.SendTransaction(ctx, signed); err == nil {
			s.nonce++
			return signed, nil
		}
		if attempt == 0 && isNonceTooLow(err) {
			n, rerr := ec.PendingNonceAt(ctx, s.Address())
			if rerr != nil {
				return nil, fmt.Errorf("send tx: %w (nonce re-sync: %v)", err, rerr)
			}
			s.nonce = n
			continue
		}
		return nil, fmt.Errorf("send tx: %w", err)
	}
	return nil, fmt.Errorf("send tx: nonce re-sync did not resolve the error")
}

// WithNonce runs broadcast under this signer's nonce counter and returns the transaction it sent.
//
// It exists for go-ethereum's generated deployers, which sign and broadcast internally: they read
// the nonce from bind.TransactOpts, and an opts with a nil Nonce makes bind fetch PendingNonceAt on
// its own — a private counter again, outside everything signAndSend serializes. Passing the nonce
// this method supplies puts a deploy back in line with every other submission from the account.
//
// broadcast must set the nonce it is given on its TransactOpts and return the broadcast
// transaction. As in signAndSend, a stale counter is re-synced from PendingNonceAt and the
// broadcast is retried once.
func (s *Signer) WithNonce(
	ctx context.Context,
	ec *ethclient.Client,
	broadcast func(nonce uint64) (*types.Transaction, error),
) (*types.Transaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.nonceInit {
		n, err := ec.PendingNonceAt(ctx, s.Address())
		if err != nil {
			return nil, fmt.Errorf("nonce: %w", err)
		}
		s.nonce = n
		s.nonceInit = true
	}

	for attempt := 0; attempt < 2; attempt++ {
		tx, err := broadcast(s.nonce)
		if err == nil {
			s.nonce++
			return tx, nil
		}
		if attempt == 0 && isNonceTooLow(err) {
			n, rerr := ec.PendingNonceAt(ctx, s.Address())
			if rerr != nil {
				return nil, fmt.Errorf("broadcast: %w (nonce re-sync: %v)", err, rerr)
			}
			s.nonce = n
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("broadcast: nonce re-sync did not resolve the error")
}

// isNonceTooLow reports whether a SendTransaction error indicates the account nonce on the
// node has advanced past the one we used — e.g. after a node restart or an out-of-band tx.
func isNonceTooLow(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "nonce too low") ||
		strings.Contains(msg, "nonce too high") ||
		strings.Contains(msg, "replacement transaction underpriced")
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

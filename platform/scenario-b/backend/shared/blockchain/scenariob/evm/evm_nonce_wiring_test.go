// SPDX-License-Identifier: Apache-2.0

package evm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// --- the wiring: does a receipt wait that dies on its deadline actually invalidate the counter? ---
//
// ShouldResyncNonce and MarkNonceStale are each covered on their own, but the line that JOINS them
// lives in submitRawInternal, behind a WaitForReceipt that takes a concrete *ethclient.Client. An
// interface fake cannot reach it, and the original defect WAS an absent wiring — the counter never
// re-initialised — with a symptom that is silent by construction: the broadcast succeeds, nothing
// mines, no error names the nonce. So the join is tested here against a real ethclient speaking to a
// stub JSON-RPC node, the same shape the payment-orchestrator's besu tests already use.
//
// Delete the MarkNonceStale call in submitRawInternal and the first test below fails.

// stubNode answers the handful of JSON-RPC methods a submission touches. It mines nothing unless
// told to, which is exactly the state a drifted counter produces on Besu: the transaction is
// accepted into the pool and waits for a predecessor that will never exist.
type stubNode struct {
	mu sync.Mutex
	// nonce is what eth_getTransactionCount reports; nonceReads counts how often it was asked,
	// which is how a re-seed is observed.
	nonce      uint64
	nonceReads int
	mines      bool
	sent       []*types.Transaction
}

func (n *stubNode) start(t *testing.T) *ethclient.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("stub node: read body: %v", err)
			return
		}
		var req struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
			ID     json.RawMessage   `json:"id"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("stub node: unmarshal request: %v", err)
			return
		}
		result, err := n.answer(req.Method, req.Params)
		if err != nil {
			t.Errorf("stub node: %v", err)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      json.RawMessage(req.ID),
			"result":  result,
		})
	}))
	t.Cleanup(srv.Close)

	ec, err := ethclient.Dial(srv.URL)
	if err != nil {
		t.Fatalf("dial stub node: %v", err)
	}
	t.Cleanup(ec.Close)
	return ec
}

func (n *stubNode) answer(method string, params []json.RawMessage) (any, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	switch method {
	case "eth_chainId":
		return "0x539", nil
	case "eth_gasPrice":
		return "0x0", nil
	case "eth_estimateGas":
		return "0x5208", nil
	case "eth_getCode":
		return "0x600160005260206000f3", nil
	case "eth_blockNumber":
		return "0x1", nil
	case "eth_getTransactionCount":
		n.nonceReads++
		return hexutil.Uint64(n.nonce).String(), nil
	case "eth_sendRawTransaction":
		var raw string
		if err := json.Unmarshal(params[0], &raw); err != nil {
			return nil, err
		}
		blob, err := hexutil.Decode(raw)
		if err != nil {
			return nil, err
		}
		tx := new(types.Transaction)
		if err := tx.UnmarshalBinary(blob); err != nil {
			return nil, err
		}
		n.sent = append(n.sent, tx)
		return tx.Hash().Hex(), nil
	case "eth_getTransactionReceipt":
		if !n.mines {
			return nil, nil // the pool holds it; no receipt will ever be written
		}
		var hash common.Hash
		if err := json.Unmarshal(params[0], &hash); err != nil {
			return nil, err
		}
		return successReceipt(hash), nil
	default:
		return nil, errors.New("unexpected JSON-RPC method " + method)
	}
}

// lastNonce is the nonce of the most recent submission the node accepted.
func (n *stubNode) lastNonce(t *testing.T) uint64 {
	t.Helper()
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.sent) == 0 {
		t.Fatal("no transaction reached the node")
	}
	return n.sent[len(n.sent)-1].Nonce()
}

func (n *stubNode) reads() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.nonceReads
}

func (n *stubNode) setState(nonce uint64, mines bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.nonce = nonce
	n.mines = mines
}

func successReceipt(hash common.Hash) map[string]any {
	return map[string]any{
		"type":              "0x0",
		"status":            "0x1",
		"transactionHash":   hash.Hex(),
		"transactionIndex":  "0x0",
		"blockHash":         common.HexToHash("0xb10c").Hex(),
		"blockNumber":       "0x1",
		"cumulativeGasUsed": "0x5208",
		"gasUsed":           "0x5208",
		"effectiveGasPrice": "0x0",
		"logs":              []any{},
		"logsBloom":         "0x" + strings.Repeat("0", 512),
		"contractAddress":   nil,
	}
}

// shortReceiptWait bounds the wait so a test that must time out costs milliseconds, not 90 s.
func shortReceiptWait(t *testing.T) {
	t.Helper()
	previous := ReceiptWaitTimeout
	ReceiptWaitTimeout = 150 * time.Millisecond
	t.Cleanup(func() { ReceiptWaitTimeout = previous })
}

var wiringTarget = common.HexToAddress("0x00000000000000000000000000000000000000aa")

func newWiringSigner(t *testing.T) *Signer {
	t.Helper()
	// NewSigner, not SharedSigner: these tests drift a counter on purpose and must not hand a
	// poisoned one to another test in the same process.
	s, err := NewSigner(newKeyHex(t), big.NewInt(1337))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	return s
}

func TestSubmitRawTxReceipt_ReceiptWaitTimeoutMarksTheCounterStale(t *testing.T) {
	shortReceiptWait(t)
	node := &stubNode{nonce: 7}
	ec := node.start(t)
	s := newWiringSigner(t)

	_, _, err := SubmitRawTxReceipt(context.Background(), ec, s, wiringTarget, []byte{0x01}, "drifted mint")
	if err == nil {
		t.Fatal("a submission the node never mines must fail")
	}
	// The failure has to come from the wait, not the broadcast: a drifted counter produces no
	// broadcast error at all, which is why the wait is the only available trigger.
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected the receipt wait to die on its deadline, got: %v", err)
	}
	if node.lastNonce(t) != 7 {
		t.Fatalf("submission used nonce %d, want the seeded 7", node.lastNonce(t))
	}

	s.mu.Lock()
	stillTrusted := s.nonceInit
	s.mu.Unlock()
	if stillTrusted {
		t.Fatal("counter still claims to be in step after a receipt wait died on its deadline — " +
			"submitRawInternal did not invalidate it, so every later submission queues behind a nonce that will never be reached")
	}

	// The recovery: the chain came back at 0 under a live process, and the next submission must
	// re-read rather than carry on from 8.
	node.setState(0, true)
	if _, _, err := SubmitRawTxReceipt(context.Background(), ec, s, wiringTarget, []byte{0x01}, "recovery"); err != nil {
		t.Fatalf("the submission after the timeout did not recover: %v", err)
	}
	if got := node.lastNonce(t); got != 0 {
		t.Fatalf("the recovering submission used nonce %d, want 0 — the counter re-seeded from the drift, not from the chain", got)
	}
	if got := node.reads(); got != 2 {
		t.Fatalf("eth_getTransactionCount was called %d times, want 2 (one seed, one re-seed)", got)
	}
}

func TestSubmitRawTxReceipt_MinedReceiptKeepsTheCounter(t *testing.T) {
	node := &stubNode{nonce: 7, mines: true}
	ec := node.start(t)
	s := newWiringSigner(t)

	for i := 0; i < 2; i++ {
		if _, _, err := SubmitRawTxReceipt(context.Background(), ec, s, wiringTarget, []byte{0x01}, "mint"); err != nil {
			t.Fatalf("submission %d: %v", i, err)
		}
	}
	if got := node.lastNonce(t); got != 8 {
		t.Fatalf("second submission used nonce %d, want 8", got)
	}
	if got := node.reads(); got != 1 {
		t.Fatalf("eth_getTransactionCount was called %d times, want 1 — a mined submission must not invalidate the counter", got)
	}
}

func TestSubmitRawTxReceipt_CallerCancellationKeepsTheCounter(t *testing.T) {
	// A generous timeout on purpose: the cancellation below must be what ends the wait, and a
	// tight bound would race it on a loaded machine and turn this into the deadline case.
	previous := ReceiptWaitTimeout
	ReceiptWaitTimeout = time.Minute
	t.Cleanup(func() { ReceiptWaitTimeout = previous })

	node := &stubNode{nonce: 7}
	ec := node.start(t)
	s := newWiringSigner(t)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	_, _, err := SubmitRawTxReceipt(ctx, ec, s, wiringTarget, []byte{0x01}, "cancelled mint")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected the wait to end in cancellation, got: %v", err)
	}

	s.mu.Lock()
	stillTrusted := s.nonceInit
	s.mu.Unlock()
	if !stillTrusted {
		t.Fatal("a cancelled caller invalidated the counter — the transaction may still mine, and its nonce is spent")
	}
}

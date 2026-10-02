// SPDX-License-Identifier: Apache-2.0

package app

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

	"github.com/LACNetNetworks/cbweb3-platform/backend/shared/blockchain/scenariob/evm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// --- the two submissions this adapter broadcasts through bind ---
//
// DeployDedicatedAMM and applyDefaultFee do not go through evm.SubmitRawTx: the generated bindings
// broadcast for them, and they wait via the exported evm.WaitForReceipt. So the shared submission
// path's counter invalidation never runs for them, and each has to ask for it explicitly. A deploy
// or a setFeeBps that is never mined leaves the counter one ahead of the chain, and from then on
// every submission from this gateway's hub account queues behind a nonce that will never be reached
// — with no error naming the nonce anywhere, because the broadcast itself succeeded.
//
// The tests observe the counter from outside the evm package, where nonceInit is not visible: a
// re-seed is exactly one extra eth_getTransactionCount, so counting that call is the assertion.
// Delete either evm.ShouldResyncNonce/MarkNonceStale pair in the adapter and the timeout tests below
// fail; their mined-receipt twins pin the other side, that a healthy submission must NOT re-read.

// rpcStub answers the JSON-RPC methods a bind deploy and a bind call touch. It mines nothing unless
// told to, which is the state a drifted counter produces on Besu: the transaction is queued and
// waits for a predecessor that will never exist.
type rpcStub struct {
	mu         sync.Mutex
	nonce      uint64
	nonceReads int
	mines      bool
	sent       []*types.Transaction
}

func (s *rpcStub) start(t *testing.T) *ethclient.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("rpc stub: read body: %v", err)
			return
		}
		var req struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
			ID     json.RawMessage   `json:"id"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("rpc stub: unmarshal request: %v", err)
			return
		}
		result, err := s.answer(req.Method, req.Params)
		if err != nil {
			t.Errorf("rpc stub: %v", err)
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
		t.Fatalf("dial rpc stub: %v", err)
	}
	t.Cleanup(ec.Close)
	return ec
}

func (s *rpcStub) answer(method string, params []json.RawMessage) (any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch method {
	case "eth_chainId":
		return "0x539", nil
	case "eth_gasPrice":
		return "0x0", nil
	case "eth_estimateGas":
		return "0x5208", nil
	case "eth_getCode":
		// Non-empty: bind refuses to estimate gas for a call to an address with no code.
		return "0x600160005260206000f3", nil
	case "eth_blockNumber":
		return "0x1", nil
	case "eth_getTransactionCount":
		s.nonceReads++
		return hexutil.Uint64(s.nonce).String(), nil
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
		s.sent = append(s.sent, tx)
		return tx.Hash().Hex(), nil
	case "eth_getTransactionReceipt":
		if !s.mines {
			return nil, nil
		}
		var hash common.Hash
		if err := json.Unmarshal(params[0], &hash); err != nil {
			return nil, err
		}
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
		}, nil
	default:
		return nil, errors.New("unexpected JSON-RPC method " + method)
	}
}

func (s *rpcStub) reads() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nonceReads
}

func (s *rpcStub) lastNonce(t *testing.T) uint64 {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.sent) == 0 {
		t.Fatal("no transaction reached the node")
	}
	return s.sent[len(s.sent)-1].Nonce()
}

func (s *rpcStub) setState(nonce uint64, mines bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nonce = nonce
	s.mines = mines
}

// newPairRegistryClientForStub wires a client onto the stub node. The signer is a NewSigner, not the
// process-wide SharedSigner: these tests drift a counter on purpose and must not hand a poisoned one
// to another test in this binary.
func newPairRegistryClientForStub(t *testing.T, stub *rpcStub) *PairRegistryClient {
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	signer, err := evm.NewSigner(common.Bytes2Hex(crypto.FromECDSA(key)), big.NewInt(1337))
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	parsed, err := evm.ParseABI(pairRegistryABI)
	if err != nil {
		t.Fatalf("parse ABI: %v", err)
	}
	return &PairRegistryClient{
		contract:         common.HexToAddress("0x00000000000000000000000000000000000000c1"),
		ec:               stub.start(t),
		parsed:           parsed,
		signer:           signer,
		timeout:          5 * time.Second,
		identityRegistry: common.HexToAddress("0x00000000000000000000000000000000000000d1"),
	}
}

// shortReceiptWait bounds the wait so a test that must time out costs milliseconds, not 90 s.
func shortReceiptWait(t *testing.T) {
	t.Helper()
	previous := evm.ReceiptWaitTimeout
	evm.ReceiptWaitTimeout = 150 * time.Millisecond
	t.Cleanup(func() { evm.ReceiptWaitTimeout = previous })
}

func TestDeployDedicatedAMM_ReceiptWaitTimeoutResyncsTheCounter(t *testing.T) {
	shortReceiptWait(t)
	// The fee call is a separate submission with its own invalidation; silence it here so this test
	// speaks only about the deploy.
	t.Setenv("AMM_DEFAULT_FEE_BPS", "0")

	stub := &rpcStub{nonce: 5}
	c := newPairRegistryClientForStub(t, stub)
	tokenA := "0x0000000000000000000000000000000000000a01"
	tokenB := "0x0000000000000000000000000000000000000b01"

	if _, err := c.DeployDedicatedAMM(context.Background(), tokenA, tokenB); err == nil {
		t.Fatal("a deploy the node never mines must fail")
	} else if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected the receipt wait to die on its deadline, got: %v", err)
	}
	if got := stub.reads(); got != 1 {
		t.Fatalf("eth_getTransactionCount was called %d times seeding the counter, want 1", got)
	}

	// The chain came back at 0 under a live process: the next submission must re-read rather than
	// carry on from 6.
	stub.setState(0, true)
	if _, err := c.DeployDedicatedAMM(context.Background(), tokenA, tokenB); err != nil {
		t.Fatalf("the deploy after the timeout did not recover: %v", err)
	}
	if got := stub.reads(); got != 2 {
		t.Fatalf("eth_getTransactionCount was called %d times, want 2 — the deploy path never marked the counter stale, "+
			"so every later submission from this account queues behind a nonce that will never be reached", got)
	}
	if got := stub.lastNonce(t); got != 0 {
		t.Fatalf("the recovering deploy used nonce %d, want 0 — the counter re-seeded from the drift, not from the chain", got)
	}
}

func TestDeployDedicatedAMM_MinedReceiptKeepsTheCounter(t *testing.T) {
	t.Setenv("AMM_DEFAULT_FEE_BPS", "0")

	stub := &rpcStub{nonce: 5, mines: true}
	c := newPairRegistryClientForStub(t, stub)
	tokenA := "0x0000000000000000000000000000000000000a01"
	tokenB := "0x0000000000000000000000000000000000000b01"

	for i := 0; i < 2; i++ {
		if _, err := c.DeployDedicatedAMM(context.Background(), tokenA, tokenB); err != nil {
			t.Fatalf("deploy %d: %v", i, err)
		}
	}
	if got := stub.reads(); got != 1 {
		t.Fatalf("eth_getTransactionCount was called %d times, want 1 — a mined deploy must not invalidate the counter", got)
	}
	if got := stub.lastNonce(t); got != 6 {
		t.Fatalf("second deploy used nonce %d, want 6", got)
	}
}

func TestApplyDefaultFee_ReceiptWaitTimeoutResyncsTheCounter(t *testing.T) {
	shortReceiptWait(t)

	stub := &rpcStub{nonce: 9}
	c := newPairRegistryClientForStub(t, stub)
	amm := common.HexToAddress("0x0000000000000000000000000000000000000acc")

	// applyDefaultFee is best-effort by design — the AMM already exists on-chain, so it logs and
	// returns rather than failing the propose. The counter still has to be invalidated.
	c.applyDefaultFee(context.Background(), amm)
	if got := stub.reads(); got != 1 {
		t.Fatalf("eth_getTransactionCount was called %d times seeding the counter, want 1", got)
	}

	stub.setState(0, true)
	c.applyDefaultFee(context.Background(), amm)
	if got := stub.reads(); got != 2 {
		t.Fatalf("eth_getTransactionCount was called %d times, want 2 — setFeeBps never marked the counter stale "+
			"after its receipt wait died on its deadline", got)
	}
	if got := stub.lastNonce(t); got != 0 {
		t.Fatalf("the recovering setFeeBps used nonce %d, want 0 — the counter re-seeded from the drift, not from the chain", got)
	}
}

func TestApplyDefaultFee_MinedReceiptKeepsTheCounter(t *testing.T) {
	stub := &rpcStub{nonce: 9, mines: true}
	c := newPairRegistryClientForStub(t, stub)
	amm := common.HexToAddress("0x0000000000000000000000000000000000000acc")

	c.applyDefaultFee(context.Background(), amm)
	c.applyDefaultFee(context.Background(), amm)

	if got := stub.reads(); got != 1 {
		t.Fatalf("eth_getTransactionCount was called %d times, want 1 — a confirmed setFeeBps must not invalidate the counter", got)
	}
	if got := stub.lastNonce(t); got != 10 {
		t.Fatalf("second setFeeBps used nonce %d, want 10", got)
	}
}

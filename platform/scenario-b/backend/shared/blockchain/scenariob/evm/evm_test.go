// SPDX-License-Identifier: Apache-2.0

package evm

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

func newKeyHex(t *testing.T) string {
	t.Helper()
	key, err := gethcrypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return hex.EncodeToString(gethcrypto.FromECDSA(key))
}

func TestNewSigner(t *testing.T) {
	keyHex := newKeyHex(t)

	t.Run("success", func(t *testing.T) {
		s, err := NewSigner("0x"+keyHex, big.NewInt(1337))
		if err != nil {
			t.Fatalf("NewSigner: %v", err)
		}
		if (s.Address() == common.Address{}) {
			t.Fatal("expected non-zero address")
		}
		if s.ChainID().Cmp(big.NewInt(1337)) != 0 {
			t.Fatalf("chainID=%s", s.ChainID())
		}
		// ChainID returns a copy; mutating it must not affect the signer.
		got := s.ChainID()
		got.SetInt64(99)
		if s.ChainID().Cmp(big.NewInt(1337)) != 0 {
			t.Fatal("ChainID must return a defensive copy")
		}
	})

	t.Run("empty_key", func(t *testing.T) {
		if _, err := NewSigner("  ", big.NewInt(1)); err == nil {
			t.Fatal("expected error for empty key")
		}
	})

	t.Run("nil_chain_id", func(t *testing.T) {
		if _, err := NewSigner(keyHex, nil); err == nil {
			t.Fatal("expected error for nil chainID")
		}
	})

	t.Run("non_positive_chain_id", func(t *testing.T) {
		if _, err := NewSigner(keyHex, big.NewInt(0)); err == nil {
			t.Fatal("expected error for zero chainID")
		}
	})

	t.Run("invalid_key", func(t *testing.T) {
		if _, err := NewSigner("zzzz", big.NewInt(1)); err == nil {
			t.Fatal("expected error for invalid key")
		}
	})
}

func TestSigner_TransactOpts(t *testing.T) {
	s, err := NewSigner(newKeyHex(t), big.NewInt(1337))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	ctx := context.Background()
	opts, err := s.TransactOpts(ctx)
	if err != nil {
		t.Fatalf("TransactOpts: %v", err)
	}
	if opts.From != s.Address() {
		t.Fatalf("opts.From=%s want %s", opts.From.Hex(), s.Address().Hex())
	}
	if opts.Context != ctx {
		t.Fatal("opts.Context not set")
	}
}

func TestParseABI(t *testing.T) {
	const raw = `[{"type":"function","name":"foo","inputs":[],"outputs":[{"type":"uint256"}]}]`
	t.Run("valid", func(t *testing.T) {
		parsed, err := ParseABI("  " + raw + "  ")
		if err != nil {
			t.Fatalf("ParseABI: %v", err)
		}
		if _, ok := parsed.Methods["foo"]; !ok {
			t.Fatal("method foo missing")
		}
	})
	t.Run("invalid", func(t *testing.T) {
		if _, err := ParseABI("not json"); err == nil {
			t.Fatal("expected error for invalid ABI")
		}
	})
}

func TestDial_Errors(t *testing.T) {
	t.Run("empty_url", func(t *testing.T) {
		if _, err := Dial(context.Background(), "", time.Second); err == nil {
			t.Fatal("expected error for empty URL")
		}
	})
	t.Run("unreachable", func(t *testing.T) {
		// ethclient.DialContext for an HTTP URL is lazy and may not error here;
		// use an obviously malformed scheme to force a dial error.
		if _, err := Dial(context.Background(), "://bad", 100*time.Millisecond); err == nil {
			t.Fatal("expected error for malformed URL")
		}
	})
}

func TestSigner_NonceInit(t *testing.T) {
	s, err := NewSigner(newKeyHex(t), big.NewInt(1337))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	if s.nonceInit {
		t.Fatal("nonceInit should start false; first signAndSend seeds it from PendingNonceAt")
	}
}

func TestIsNonceTooLow(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"nonce too low", true},
		{"nonce too high", true},
		{"replacement transaction underpriced", true},
		{"NONCE TOO LOW", true}, // case-insensitive
		{"insufficient funds", false},
		{"known transaction", false},
		{"gas limit exceeded", false},
	}
	for _, tc := range tests {
		if got := isNonceTooLow(fmt.Errorf("%s", tc.msg)); got != tc.want {
			t.Errorf("isNonceTooLow(%q) = %v, want %v", tc.msg, got, tc.want)
		}
	}
}

func TestCopyOutput(t *testing.T) {
	t.Run("big_int", func(t *testing.T) {
		var dst big.Int
		if err := copyOutput(big.NewInt(42), &dst); err != nil || dst.Int64() != 42 {
			t.Fatalf("err=%v dst=%s", err, dst.String())
		}
	})
	t.Run("bool", func(t *testing.T) {
		var dst bool
		if err := copyOutput(true, &dst); err != nil || !dst {
			t.Fatalf("err=%v dst=%v", err, dst)
		}
	})
	t.Run("address", func(t *testing.T) {
		var dst common.Address
		src := common.HexToAddress("0x1111111111111111111111111111111111111111")
		if err := copyOutput(src, &dst); err != nil || dst != src {
			t.Fatalf("err=%v dst=%s", err, dst.Hex())
		}
	})
	t.Run("bytes32", func(t *testing.T) {
		var dst [32]byte
		var src [32]byte
		src[0] = 0xAB
		if err := copyOutput(src, &dst); err != nil || dst != src {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("string", func(t *testing.T) {
		var dst string
		if err := copyOutput("hi", &dst); err != nil || dst != "hi" {
			t.Fatalf("err=%v dst=%q", err, dst)
		}
	})
	t.Run("type_mismatch", func(t *testing.T) {
		var dst big.Int
		if err := copyOutput("not-a-bigint", &dst); err == nil {
			t.Fatal("expected type mismatch error")
		}
	})
	t.Run("unsupported_dst", func(t *testing.T) {
		var dst float64
		if err := copyOutput(1.0, &dst); err == nil {
			t.Fatal("expected unsupported type error")
		}
	})
}

// --- raw-input submission ---
//
// Three payment-orchestrator clients (tCeBM, fiat, FX agreement) each built their own transactor
// from the same operator key and fetched PendingNonceAt themselves, bypassing this package's
// serialized counter. With the same key on the same chain, two concurrent submissions could claim
// the same nonce and one would be replaced — the failure this Signer exists to prevent, and the
// same one that motivated giving the CB's relayer its own identity.
//
// They pack their own calldata, so converging them needs a raw-input entry point rather than a
// method+args one. These tests pin the contract of that entry point; the nonce serialization itself
// is exercised by the shared path they now share.

// --- one nonce counter per account ---
//
// The regression these pin: a Signer owns a nonce counter, so a second Signer on the same key is a
// second counter on one account. Two of them submitting concurrently claim the same nonce, and on a
// zero-gas chain the later transaction replaces the earlier one — the replaced hash never mines and
// its caller waits on a receipt that will never exist. That is what hung a deposit approval when the
// payments clients and the bridge relayer each held their own Signer for the CB's spoke key.

func TestSharedSigner_SameAccountAndChainShareOneInstance(t *testing.T) {
	keyHex := newKeyHex(t)

	first, err := SharedSigner(keyHex, big.NewInt(1337))
	if err != nil {
		t.Fatalf("SharedSigner: %v", err)
	}
	// The "0x" prefix is a spelling of the same key, not a different account.
	second, err := SharedSigner("0x"+keyHex, big.NewInt(1337))
	if err != nil {
		t.Fatalf("SharedSigner: %v", err)
	}
	if first != second {
		t.Fatal("same key and chain must yield ONE signer: two instances are two nonce counters on one account")
	}
}

func TestSharedSigner_SeparateAccountsAndChainsStaySeparate(t *testing.T) {
	keyHex := newKeyHex(t)

	spoke, err := SharedSigner(keyHex, big.NewInt(1338))
	if err != nil {
		t.Fatalf("SharedSigner: %v", err)
	}
	// The same key on another chain is another account state: nonces are per (account, chain).
	hub, err := SharedSigner(keyHex, big.NewInt(1339))
	if err != nil {
		t.Fatalf("SharedSigner: %v", err)
	}
	if spoke == hub {
		t.Fatal("the same key on two chains must not share a counter")
	}

	other, err := SharedSigner(newKeyHex(t), big.NewInt(1338))
	if err != nil {
		t.Fatalf("SharedSigner: %v", err)
	}
	if other == spoke {
		t.Fatal("different keys must not share a counter")
	}
}

func TestSharedSigner_ConcurrentConstructionYieldsOneInstance(t *testing.T) {
	keyHex := newKeyHex(t)
	const callers = 16

	signers := make([]*Signer, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s, err := SharedSigner(keyHex, big.NewInt(1337))
			if err != nil {
				t.Errorf("SharedSigner: %v", err)
				return
			}
			signers[i] = s
		}(i)
	}
	wg.Wait()

	for i, s := range signers {
		if s == nil || s != signers[0] {
			t.Fatalf("caller %d got a different signer — construction order must not decide who owns the counter", i)
		}
	}
}

func TestSharedSigner_CounterIsSharedAcrossHolders(t *testing.T) {
	keyHex := newKeyHex(t)

	payments, err := SharedSigner(keyHex, big.NewInt(1337))
	if err != nil {
		t.Fatalf("SharedSigner: %v", err)
	}
	relayer, err := SharedSigner(keyHex, big.NewInt(1337))
	if err != nil {
		t.Fatalf("SharedSigner: %v", err)
	}
	// Seed the counter as a first submission would, without needing a chain.
	payments.nonce = 15
	payments.nonceInit = true

	sent, err := payments.WithNonce(context.Background(), nil, func(nonce uint64) (*types.Transaction, error) {
		if nonce != 15 {
			t.Errorf("first submission used nonce %d, want 15", nonce)
		}
		return types.NewTransaction(nonce, common.HexToAddress("0x1"), big.NewInt(0), 21000, big.NewInt(0), nil), nil
	})
	if err != nil || sent == nil {
		t.Fatalf("WithNonce: err=%v tx=%v", err, sent)
	}

	// The relayer holds the same instance, so it must see 15 as spent — this is the exact step
	// that failed in production, where its private counter handed out 15 a second time.
	if _, err := relayer.WithNonce(context.Background(), nil, func(nonce uint64) (*types.Transaction, error) {
		if nonce != 16 {
			t.Errorf("second holder used nonce %d, want 16 — the counter is not shared", nonce)
		}
		return types.NewTransaction(nonce, common.HexToAddress("0x1"), big.NewInt(0), 21000, big.NewInt(0), nil), nil
	}); err != nil {
		t.Fatalf("WithNonce: %v", err)
	}
}

func TestSigner_WithNonce_FailedBroadcastDoesNotConsumeTheNonce(t *testing.T) {
	s, err := NewSigner(newKeyHex(t), big.NewInt(1337))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	s.nonce = 4
	s.nonceInit = true

	// A broadcast that never reached the node leaves the nonce unspent; consuming it anyway would
	// leave a permanent gap that stalls every later transaction from the account.
	if _, err := s.WithNonce(context.Background(), nil, func(uint64) (*types.Transaction, error) {
		return nil, errors.New("insufficient funds")
	}); err == nil {
		t.Fatal("expected the broadcast error to surface")
	}
	if s.nonce != 4 {
		t.Fatalf("nonce advanced to %d after a failed broadcast, want 4", s.nonce)
	}
}

// --- bounding the receipt wait ---

func TestReceiptWaitContext(t *testing.T) {
	t.Run("imposes_a_deadline_when_the_caller_has_none", func(t *testing.T) {
		ctx, cancel := receiptWaitContext(context.Background())
		defer cancel()
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("an unbounded wait is what turns a dropped transaction into a permanent hang")
		}
	})

	t.Run("keeps_the_caller_deadline", func(t *testing.T) {
		callerCtx, cancelCaller := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelCaller()
		callerDeadline, _ := callerCtx.Deadline()

		ctx, cancel := receiptWaitContext(callerCtx)
		defer cancel()
		got, ok := ctx.Deadline()
		if !ok || !got.Equal(callerDeadline) {
			t.Fatalf("caller deadline %v was replaced by %v", callerDeadline, got)
		}
	})

	t.Run("disabled_when_the_timeout_is_not_positive", func(t *testing.T) {
		previous := ReceiptWaitTimeout
		ReceiptWaitTimeout = 0
		defer func() { ReceiptWaitTimeout = previous }()

		ctx, cancel := receiptWaitContext(context.Background())
		defer cancel()
		if _, ok := ctx.Deadline(); ok {
			t.Fatal("a non-positive timeout must leave the wait unbounded, as an explicit opt-out")
		}
	})
}

func TestSubmitRawTxReceipt_RequiresASigner(t *testing.T) {
	if _, _, err := SubmitRawTxReceipt(context.Background(), nil, nil,
		common.HexToAddress("0x1"), []byte{0x01}, "mint"); err == nil {
		t.Fatal("expected a nil signer to be refused rather than panicking")
	}
}

func TestSubmitRawTxReceipt_RequiresInput(t *testing.T) {
	s, err := NewSigner(newKeyHex(t), big.NewInt(1337))
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	if _, _, err := SubmitRawTxReceipt(context.Background(), nil, s,
		common.HexToAddress("0x1"), nil, "mint"); err == nil {
		t.Fatal("expected empty calldata to be refused: it would send a bare value transfer")
	}
}

// --- re-syncing the counter after a receipt wait dies on its deadline ---
//
// The regression: SharedSigner makes the counter live for the process lifetime and nonceInit is set
// exactly once, so it is re-read only when SendTransaction returns a nonce-too-low error. Besu
// QUEUES a future-nonce transaction instead of rejecting it, so a counter that has drifted AHEAD of
// chain state — a nuke-and-redeploy of the Besu layer with the backend containers still up — produces
// no broadcast error at all: every submission is accepted, none is ever mined, and each caller fails
// after ReceiptWaitTimeout with no path back to a correct counter. Before the shared registry this
// recovered by accident, because a freshly constructed client re-read PendingNonceAt; a process-wide
// instance never re-initialises.

type fakeNonceSource struct {
	nonce uint64
	calls int
	err   error
}

func (f *fakeNonceSource) PendingNonceAt(context.Context, common.Address) (uint64, error) {
	f.calls++
	if f.err != nil {
		return 0, f.err
	}
	return f.nonce, nil
}

func broadcastExpecting(t *testing.T, want uint64) func(uint64) (*types.Transaction, error) {
	t.Helper()
	return func(nonce uint64) (*types.Transaction, error) {
		if nonce != want {
			t.Errorf("submission used nonce %d, want %d", nonce, want)
		}
		return types.NewTransaction(nonce, common.HexToAddress("0x1"), big.NewInt(0), 21000, big.NewInt(0), nil), nil
	}
}

func TestShouldResyncNonce(t *testing.T) {
	// WaitForReceipt wraps whatever bind.WaitMined returned, so the classification has to survive
	// wrapping — matching on the error text would not.
	wrapped := func(err error) error {
		return fmt.Errorf("wait mined (mint tx=0xabc): %w", err)
	}

	tests := []struct {
		name string
		err  error
		want bool
		why  string
	}{
		{
			name: "receipt_arrived",
			err:  nil,
			want: false,
			why:  "a mined transaction consumed its nonce; the counter is in step",
		},
		{
			name: "deadline_expired",
			err:  wrapped(context.DeadlineExceeded),
			want: true,
			why:  "no receipt within the deadline is the one symptom a drifted counter produces on Besu",
		},
		{
			name: "caller_cancelled",
			err:  wrapped(context.Canceled),
			want: false,
			why:  "a cancelled request says nothing about the counter — the transaction may mine a moment later",
		},
		{
			name: "reverted_on_chain",
			err:  errors.New("mint: transaction reverted on-chain (tx=0xabc)"),
			want: false,
			why:  "a revert means it mined: the nonce was spent and the counter is correct",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShouldResyncNonce(tc.err); got != tc.want {
				t.Fatalf("ShouldResyncNonce = %v, want %v — %s", got, tc.want, tc.why)
			}
		})
	}
}

func TestSigner_MarkNonceStale_MakesTheNextSubmissionReseed(t *testing.T) {
	s, err := NewSigner(newKeyHex(t), big.NewInt(1337))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	src := &fakeNonceSource{nonce: 41}

	// First submission seeds the counter from the node.
	if _, err := s.WithNonce(context.Background(), src, broadcastExpecting(t, 41)); err != nil {
		t.Fatalf("WithNonce: %v", err)
	}
	if src.calls != 1 {
		t.Fatalf("PendingNonceAt called %d times seeding the counter, want 1", src.calls)
	}

	// The second trusts the local counter: re-reading on every submission is what the shared
	// counter exists to avoid.
	if _, err := s.WithNonce(context.Background(), src, broadcastExpecting(t, 42)); err != nil {
		t.Fatalf("WithNonce: %v", err)
	}
	if src.calls != 1 {
		t.Fatalf("PendingNonceAt called %d times, want 1 — a healthy submission must not re-read", src.calls)
	}

	// The chain was reset under a live process: the node's pending nonce is 0 again while the local
	// counter sits at 43. Nothing in the broadcast path reports this, so the receipt timeout marking
	// the counter stale is the only way back.
	src.nonce = 0
	s.MarkNonceStale()

	if _, err := s.WithNonce(context.Background(), src, broadcastExpecting(t, 0)); err != nil {
		t.Fatalf("WithNonce: %v", err)
	}
	if src.calls != 2 {
		t.Fatalf("PendingNonceAt called %d times after MarkNonceStale, want 2 — the counter never re-synced", src.calls)
	}
}

func TestSigner_MarkNonceStale_IsIdempotentOnAHealthyNode(t *testing.T) {
	// Not every expiry that reaches ShouldResyncNonce is a drifted counter: receiptWaitContext keeps
	// a CALLER-supplied deadline when there is one (TestReceiptWaitContext/keeps_the_caller_deadline),
	// so a caller that bounds its own request short marks the counter stale on a node that is fine.
	// What makes that harmless is not luck — it is that the recovery is idempotent, which is this
	// test. PendingNonceAt counts CONSECUTIVE pending transactions, so a healthy node reports the
	// value the in-flight submission already advanced to: the re-seed lands where the counter
	// already was and only costs one RPC.
	s, err := NewSigner(newKeyHex(t), big.NewInt(1337))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	src := &fakeNonceSource{nonce: 41}

	if _, err := s.WithNonce(context.Background(), src, broadcastExpecting(t, 41)); err != nil {
		t.Fatalf("WithNonce: %v", err)
	}

	// The healthy node's view after that submission: nonce 41 is pending, so the next one is 42 —
	// exactly what the local counter now holds.
	src.nonce = 42
	s.MarkNonceStale()

	if _, err := s.WithNonce(context.Background(), src, broadcastExpecting(t, 42)); err != nil {
		t.Fatalf("WithNonce after a spurious invalidation: %v", err)
	}
	if src.calls != 2 {
		t.Fatalf("PendingNonceAt called %d times, want 2 — the re-seed is the whole cost of a spurious invalidation", src.calls)
	}
	if s.nonce != 43 {
		t.Fatalf("counter at %d, want 43 — a spurious invalidation must not move it", s.nonce)
	}
}

func TestSigner_MarkNonceStale_KeepsTheFailureOffTheHotPath(t *testing.T) {
	s, err := NewSigner(newKeyHex(t), big.NewInt(1337))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	s.nonce = 7
	s.nonceInit = true

	// Marking must not itself talk to the node: it runs on a failure path where the node is, by
	// hypothesis, already not answering in time. It only drops the claim of being in step.
	s.MarkNonceStale()

	if s.nonceInit {
		t.Fatal("counter still claims to be in step with the chain")
	}
	if s.nonce != 7 {
		t.Fatalf("nonce = %d, want 7 — marking stale must not invent a value; the next submission reads one", s.nonce)
	}
}

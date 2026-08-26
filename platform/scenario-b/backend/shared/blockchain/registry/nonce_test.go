// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// nonceClient builds a BesuClient whose only live dependency — the node nonce read — is
// replaced, so sendTx can be exercised without a chain.
func nonceClient(start uint64, reads *int) *BesuClient {
	var mu sync.Mutex
	return &BesuClient{
		readNonce: func(context.Context, common.Address) (uint64, error) {
			mu.Lock()
			defer mu.Unlock()
			if reads != nil {
				*reads++
			}
			return start, nil
		},
	}
}

// stubTx stands in for a broadcast transaction; sendTx only passes it through.
func stubTx() *types.Transaction {
	return types.NewTx(&types.LegacyTx{Nonce: 0, Gas: 1, GasPrice: big.NewInt(1)})
}

// The defect: with bind.TransactOpts.Nonce left nil, go-ethereum reads PendingNonceAt per
// transaction, so concurrent writes sign the SAME nonce, the loser is replaced in the mempool
// and its caller waits forever for a receipt that is never written. Every concurrent send must
// therefore come away with a distinct nonce.
func TestSendTxGivesConcurrentWritesDistinctNonces(t *testing.T) {
	const writers = 8
	reads := 0
	b := nonceClient(41, &reads)

	var mu sync.Mutex
	seen := map[uint64]int{}

	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			opts := &bind.TransactOpts{From: common.HexToAddress("0xabc")}
			_, err := b.sendTx(context.Background(), opts, "probe", func(o *bind.TransactOpts) (*types.Transaction, error) {
				mu.Lock()
				seen[o.Nonce.Uint64()]++
				mu.Unlock()
				return stubTx(), nil
			})
			if err != nil {
				t.Errorf("sendTx: %v", err)
			}
		}()
	}
	wg.Wait()

	if len(seen) != writers {
		t.Errorf("expected %d distinct nonces, got %d: %v", writers, len(seen), seen)
	}
	for n, count := range seen {
		if count > 1 {
			t.Errorf("nonce %d was handed to %d concurrent writes — they would replace each other", n, count)
		}
	}
	// One counter, not one read per call: re-reading per transaction is the original defect.
	if reads != 1 {
		t.Errorf("expected the account to be read once, got %d reads", reads)
	}
}

// Nonces must be consecutive from the account's current value. A gap parks every later
// transaction in the mempool until it is filled.
func TestSendTxAssignsConsecutiveNoncesFromTheAccount(t *testing.T) {
	b := nonceClient(100, nil)
	var got []uint64
	for i := 0; i < 3; i++ {
		opts := &bind.TransactOpts{From: common.HexToAddress("0xabc")}
		if _, err := b.sendTx(context.Background(), opts, "probe", func(o *bind.TransactOpts) (*types.Transaction, error) {
			got = append(got, o.Nonce.Uint64())
			return stubTx(), nil
		}); err != nil {
			t.Fatalf("sendTx: %v", err)
		}
	}
	for i, want := range []uint64{100, 101, 102} {
		if got[i] != want {
			t.Errorf("send %d used nonce %d, want %d (got sequence %v)", i, got[i], want, got)
		}
	}
}

// A broadcast that never reached the node must not consume a nonce, or the account stalls
// behind a gap that nothing will fill.
func TestSendTxDoesNotAdvanceOnBroadcastFailure(t *testing.T) {
	b := nonceClient(7, nil)
	opts := &bind.TransactOpts{From: common.HexToAddress("0xabc")}

	if _, err := b.sendTx(context.Background(), opts, "probe", func(*bind.TransactOpts) (*types.Transaction, error) {
		return nil, errors.New("connection refused")
	}); err == nil {
		t.Fatal("expected the broadcast error to surface")
	}

	var next uint64
	if _, err := b.sendTx(context.Background(), opts, "probe", func(o *bind.TransactOpts) (*types.Transaction, error) {
		next = o.Nonce.Uint64()
		return stubTx(), nil
	}); err != nil {
		t.Fatalf("sendTx: %v", err)
	}
	if next != 7 {
		t.Errorf("after a failed broadcast the next send used nonce %d, want 7 — a consumed nonce leaves a gap", next)
	}
}

// When the node says our counter disagrees with the account, re-read once and retry. Without
// this a drifted counter — a node restart, a transaction sent out of band — never recovers.
func TestSendTxResyncsOnceWhenTheCounterHasDrifted(t *testing.T) {
	reads := 0
	b := nonceClient(500, &reads)
	opts := &bind.TransactOpts{From: common.HexToAddress("0xabc")}

	attempts := 0
	var used []uint64
	_, err := b.sendTx(context.Background(), opts, "probe", func(o *bind.TransactOpts) (*types.Transaction, error) {
		attempts++
		used = append(used, o.Nonce.Uint64())
		if attempts == 1 {
			return nil, errors.New("nonce too low")
		}
		return stubTx(), nil
	})
	if err != nil {
		t.Fatalf("sendTx: %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected one retry after the re-read, got %d attempts (nonces %v)", attempts, used)
	}
	if reads != 2 {
		t.Errorf("expected the account to be read twice (initial + re-sync), got %d", reads)
	}
}

// An error that is not about the nonce must surface immediately rather than burn the retry.
func TestSendTxDoesNotRetryUnrelatedErrors(t *testing.T) {
	reads := 0
	b := nonceClient(3, &reads)
	opts := &bind.TransactOpts{From: common.HexToAddress("0xabc")}

	attempts := 0
	_, err := b.sendTx(context.Background(), opts, "probe", func(*bind.TransactOpts) (*types.Transaction, error) {
		attempts++
		return nil, errors.New("execution reverted: caller is not a verifier")
	})
	if err == nil {
		t.Fatal("expected the error to surface")
	}
	if attempts != 1 {
		t.Errorf("expected no retry for a non-nonce error, got %d attempts", attempts)
	}
	if reads != 1 {
		t.Errorf("expected no re-read for a non-nonce error, got %d reads", reads)
	}
}

// The lock spans send(), and that is the property this pins.
//
// Every other test here would still pass if the mutex were released right after the nonce is
// assigned: the counter stays guarded either way, so the nonces would remain distinct and
// consecutive. What changes is the order the NODE sees them in — a second goroutine could take
// the next value and reach the node first, leaving a gap that stalls the account until it is
// filled. Holding a lock across network I/O reads like a performance bug to anyone who has not
// read the comment on sendTx, which is exactly why it needs a test that fails when it is
// "optimised" away.
//
// The shape: pin the first broadcast inside send() and then try to start a second. A second
// broadcast that begins while the first is still in flight means the lock was released early.
func TestSendTxHoldsTheLockAcrossTheBroadcast(t *testing.T) {
	b := nonceClient(11, nil)
	from := common.HexToAddress("0xabc")

	firstInFlight := make(chan struct{}) // the first broadcast has begun
	releaseFirst := make(chan struct{})  // let the first broadcast return
	secondCalling := make(chan struct{}) // the second goroutine is about to call sendTx
	secondReached := make(chan struct{}) // the second broadcast has begun

	var mu sync.Mutex
	var seq []string
	record := func(s string) {
		mu.Lock()
		seq = append(seq, s)
		mu.Unlock()
	}

	go func() {
		opts := &bind.TransactOpts{From: from}
		_, _ = b.sendTx(context.Background(), opts, "first", func(*bind.TransactOpts) (*types.Transaction, error) {
			record("first:start")
			close(firstInFlight)
			<-releaseFirst
			record("first:end")
			return stubTx(), nil
		})
	}()

	<-firstInFlight

	go func() {
		opts := &bind.TransactOpts{From: from}
		close(secondCalling)
		_, _ = b.sendTx(context.Background(), opts, "second", func(*bind.TransactOpts) (*types.Transaction, error) {
			record("second:start")
			close(secondReached)
			return stubTx(), nil
		})
	}()

	// The second goroutine is running, so a quiet window below means it is blocked on the
	// mutex rather than simply unscheduled.
	<-secondCalling

	select {
	case <-secondReached:
		t.Fatal("a second broadcast began while the first was still in flight: the lock was " +
			"released after assigning the nonce, so the node can receive nonces out of order")
	case <-time.After(250 * time.Millisecond):
		// Correct — blocked on the mutex.
	}

	close(releaseFirst)

	// Anti-vacuity: the second broadcast must actually happen once the lock is free. Without
	// this the test would also pass if the second goroutine had never run at all, which would
	// prove nothing about the lock.
	select {
	case <-secondReached:
	case <-time.After(5 * time.Second):
		t.Fatal("the second broadcast never ran even after the first returned — this test " +
			"proved nothing about the lock")
	}

	mu.Lock()
	got := append([]string(nil), seq...)
	mu.Unlock()
	want := []string{"first:start", "first:end", "second:start"}
	if len(got) != len(want) {
		t.Fatalf("broadcast sequence = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("broadcast sequence = %v, want %v — the second broadcast must not "+
				"interleave with the first", got, want)
		}
	}
}

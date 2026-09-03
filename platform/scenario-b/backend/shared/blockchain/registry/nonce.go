// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Every write on this client used to leave bind.TransactOpts.Nonce nil, which makes
// go-ethereum read PendingNonceAt itself, once per transaction. Two requests in flight at the
// same moment therefore read the same value and sign the same nonce: the second replaces the
// first in the mempool, the first is dropped, and its caller waits for a receipt that will
// never be written. Nothing errors and nothing is logged, so the request simply never returns
// while the chain keeps producing blocks.
//
// Reproduced on a clean stack: four concurrent registrations against one gateway, one
// answered in 3s and three hung until a client-side timeout killed them; only the winner's
// transactions were mined, and the account's latest and pending nonce were equal afterwards —
// the losers were replaced, not queued.
//
// One counter per client, advanced under a lock held across the broadcast, removes the race
// within this process. It does NOT serialise across processes: an account written by more
// than one container still collides, which is why the per-process spoke identity matters
// separately. This mirrors evm.Signer.WithNonce, which solves the same problem for the
// payment-orchestrator's signing path.

// nonceState is the shared counter for this client's signing account.
type nonceState struct {
	mu    sync.Mutex
	next  uint64
	valid bool
}

// sendTx serialises broadcast on this client's account and gives each write an explicit nonce
// from one counter. label names the call in errors, because a dropped write is diagnosed from
// the log line rather than from a stack trace.
//
// The lock is held across send() on purpose: releasing it after assigning the nonce would let
// a second goroutine take the next value and reach the node first, which reorders nonces and
// stalls the account until the gap is filled.
func (b *BesuClient) sendTx(
	ctx context.Context,
	opts *bind.TransactOpts,
	label string,
	send func(*bind.TransactOpts) (*types.Transaction, error),
) (*types.Transaction, error) {
	b.nonces.mu.Lock()
	defer b.nonces.mu.Unlock()

	if !b.nonces.valid {
		n, err := b.pendingNonceAt(ctx, opts.From)
		if err != nil {
			return nil, fmt.Errorf("registry: %s: reading nonce for %s: %w", label, opts.From.Hex(), err)
		}
		b.nonces.next = n
		b.nonces.valid = true
	}

	// Two attempts: the second runs only after re-reading the account, for the case where the
	// counter has drifted from chain state (a node restart, or a transaction sent out of band).
	for attempt := 0; attempt < 2; attempt++ {
		opts.Nonce = new(big.Int).SetUint64(b.nonces.next)
		tx, err := send(opts)
		if err == nil {
			b.nonces.next++
			return tx, nil
		}
		if attempt == 0 && nonceOutOfSync(err) {
			n, rerr := b.pendingNonceAt(ctx, opts.From)
			if rerr != nil {
				return nil, fmt.Errorf("registry: %s: %w (re-reading nonce failed: %v)", label, err, rerr)
			}
			b.nonces.next = n
			continue
		}
		// The counter is left where it is: this transaction never reached the node, so the
		// value is still the right one for the next caller.
		return nil, fmt.Errorf("registry: %s: %w", label, err)
	}
	return nil, fmt.Errorf("registry: %s: nonce re-sync did not resolve the broadcast error", label)
}

// nonceOutOfSync reports whether the node rejected a broadcast because our counter disagrees
// with the account state, which is the one error worth retrying after a re-read.
//
// Duplicated from the sibling check in the evm package rather than exported from it: three
// string matches are a smaller cost than widening that package's API for one caller.
func nonceOutOfSync(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "nonce too low") ||
		strings.Contains(msg, "nonce too high") ||
		strings.Contains(msg, "replacement transaction underpriced")
}

// pendingNonceAt reads the account's next nonce from the node. It is a method rather than a
// direct call so a test can drive sendTx without a chain: the counter's behaviour — one read
// per client, distinct increasing nonces under concurrency, a single re-read on drift — is
// what needs covering, and none of it involves the node beyond this one call.
func (b *BesuClient) pendingNonceAt(ctx context.Context, addr common.Address) (uint64, error) {
	if b.readNonce != nil {
		return b.readNonce(ctx, addr)
	}
	return b.client.PendingNonceAt(ctx, addr)
}

// SPDX-License-Identifier: Apache-2.0

// breaker_scan.go incrementally tracks the AMM's circuit-breaker events.
//
// The governance portal polls every pair's breaker status every 15 seconds, and each read
// needs two things from the chain: the transaction hash of the pair's most recent breaker
// action (so every Central Bank cites the same reference) and, while paused, the id of the
// live resume proposal (so a CB that did not propose can still co-sign). Both used to be
// answered by an `eth_getLogs` from block 0 on every call — a query whose cost grows with
// the chain, issued four times a minute per pair per open portal. Besu slows on wide ranges
// and deployments commonly cap them outright, at which point the lookup fails and the
// service quietly falls back to its own signature table, reintroducing the per-gateway
// divergence the on-chain read exists to remove.
//
// The scanner reads each block once. The first call sweeps the history in bounded chunks
// (respecting whatever range limit the node enforces) and every later call asks only for
// the blocks that have appeared since. Both questions are answered from one sweep: the four
// breaker events share a single topic0 filter, and the most recent hit is retained per
// event as well as overall.
//
// Caching the answer keyed on the paused flag would be wrong, not merely coarse:
// proposeResume() and a below-quorum signResume() are both `whenPaused` and emit events
// without changing isPaused(), so a state-keyed cache would serve the pause's hash for the
// whole propose/sign window — exactly when an operator is watching the page.
//
// QBFT has immediate finality, so a block once scanned does not change and the cursor never
// needs to walk back.
package amm

import (
	"context"
	"math/big"
	"sync"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// breakerLogSource is the subset of *ethclient.Client the scanner uses. Narrowing it here
// is what lets the sweep be tested without a node, in the same spirit as ParseSwapLogs.
type breakerLogSource interface {
	BlockNumber(ctx context.Context) (uint64, error)
	FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error)
}

// defaultBreakerChunk is the widest block range asked for in one eth_getLogs. It only
// bounds the request, never the history covered: the sweep issues as many chunks as the
// range needs. 10k is comfortably inside the default caps deployments impose.
const defaultBreakerChunk = uint64(10_000)

// breakerHit is the part of a matched log the callers need, kept so the raw logs can be
// discarded after each chunk.
type breakerHit struct {
	blockNumber uint64
	index       uint
	txHash      common.Hash
	topics      []common.Hash
}

// newerThan orders two hits the way the chain does: by block, then by position within it.
func (h breakerHit) newerThan(other breakerHit) bool {
	if h.blockNumber != other.blockNumber {
		return h.blockNumber > other.blockNumber
	}
	return h.index > other.index
}

// breakerScanner holds one contract's scan position and the most recent hit per event.
// It is safe for concurrent use: the poll fans out over pairs, and two requests can land
// on the same pair's client at once. refresh holds the lock across its RPCs on purpose —
// concurrent callers want the same blocks read, so serialising them collapses the duplicate
// work instead of issuing the sweep several times over.
type breakerScanner struct {
	contract common.Address
	chunk    uint64
	timeout  time.Duration

	mu      sync.Mutex
	swept   bool                       // whether the historical sweep has completed
	cursor  uint64                     // highest block already scanned
	byEvent map[common.Hash]breakerHit // topic0 -> most recent hit for that event
	overall *breakerHit                // most recent hit across all four events
}

func newBreakerScanner(contract common.Address, timeout time.Duration) *breakerScanner {
	return &breakerScanner{
		contract: contract,
		chunk:    defaultBreakerChunk,
		timeout:  timeout,
		byEvent:  make(map[common.Hash]breakerHit),
	}
}

// refresh brings the scanner up to the current head.
//
// Progress is committed per chunk, so a timeout or a node error partway through leaves the
// blocks already read behind the cursor and the next call resumes from there rather than
// starting over. That matters for the very first call on a long chain, which is the only
// one that can be slow: it is amortised across polls instead of failing repeatedly.
func (s *breakerScanner) refresh(ctx context.Context, src breakerLogSource) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	head, err := s.blockNumber(ctx, src)
	if err != nil {
		return err
	}

	from := uint64(0)
	if s.swept {
		if head <= s.cursor {
			return nil // nothing new since the last poll — the common case
		}
		from = s.cursor + 1
	} else if s.cursor > 0 {
		from = s.cursor + 1 // resuming an interrupted sweep
	}

	for from <= head {
		to := from + s.chunk - 1
		if to > head {
			to = head
		}
		logs, err := s.filterLogs(ctx, src, from, to)
		if err != nil {
			return err // cursor keeps the ground already covered
		}
		s.absorb(logs)
		s.cursor = to
		from = to + 1
	}
	s.swept = true
	return nil
}

// absorb folds a chunk's logs into the retained hits. Pure apart from the receiver, and
// order-independent: a log only displaces one the chain placed before it.
func (s *breakerScanner) absorb(logs []types.Log) {
	for _, lg := range logs {
		if len(lg.Topics) == 0 {
			continue
		}
		hit := breakerHit{
			blockNumber: lg.BlockNumber,
			index:       lg.Index,
			txHash:      lg.TxHash,
			topics:      lg.Topics,
		}
		if prev, ok := s.byEvent[lg.Topics[0]]; !ok || hit.newerThan(prev) {
			s.byEvent[lg.Topics[0]] = hit
		}
		if s.overall == nil || hit.newerThan(*s.overall) {
			copied := hit
			s.overall = &copied
		}
	}
}

// latestAny returns the most recent breaker action of any kind.
func (s *breakerScanner) latestAny() (breakerHit, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.overall == nil {
		return breakerHit{}, false
	}
	return *s.overall, true
}

// latestOf returns the most recent hit for one event signature.
func (s *breakerScanner) latestOf(topic common.Hash) (breakerHit, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	hit, ok := s.byEvent[topic]
	return hit, ok
}

// blockNumber and filterLogs apply the per-request timeout. It is deliberately per request
// rather than per refresh: a sweep spanning many chunks would otherwise expire against a
// budget meant for one call.
func (s *breakerScanner) blockNumber(ctx context.Context, src breakerLogSource) (uint64, error) {
	cctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	return src.BlockNumber(cctx)
}

func (s *breakerScanner) filterLogs(ctx context.Context, src breakerLogSource, from, to uint64) ([]types.Log, error) {
	cctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	return src.FilterLogs(cctx, ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(from),
		ToBlock:   new(big.Int).SetUint64(to),
		Addresses: []common.Address{s.contract},
		// A single topic0 slot holding several values is an OR, so one query covers the
		// whole breaker lifecycle and the ordering is across all four events.
		Topics: [][]common.Hash{breakerEventSigs},
	})
}

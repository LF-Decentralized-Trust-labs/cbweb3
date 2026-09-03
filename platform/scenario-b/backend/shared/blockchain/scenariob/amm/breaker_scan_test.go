// SPDX-License-Identifier: Apache-2.0

package amm

import (
	"context"
	"errors"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// fakeLogSource is a chain the scanner can read without a node: a head, a set of logs, and
// a record of every range asked for — which is what the incremental claim is proved on.
type fakeLogSource struct {
	head     uint64
	logs     []types.Log
	queries  [][2]uint64 // from,to of each FilterLogs
	failFrom uint64      // when non-zero, a query starting at this block errors
}

func (f *fakeLogSource) BlockNumber(context.Context) (uint64, error) { return f.head, nil }

func (f *fakeLogSource) FilterLogs(_ context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	from, to := q.FromBlock.Uint64(), q.ToBlock.Uint64()
	if f.failFrom != 0 && from == f.failFrom {
		return nil, errors.New("range limit exceeded")
	}
	f.queries = append(f.queries, [2]uint64{from, to})
	var out []types.Log
	for _, lg := range f.logs {
		if lg.BlockNumber >= from && lg.BlockNumber <= to {
			out = append(out, lg)
		}
	}
	return out, nil
}

func breakerLog(block uint64, index uint, sig common.Hash, tx string, extra ...common.Hash) types.Log {
	return types.Log{
		Address:     ammAddr,
		BlockNumber: block,
		Index:       index,
		TxHash:      common.HexToHash(tx),
		Topics:      append([]common.Hash{sig}, extra...),
	}
}

func newTestScanner(chunk uint64) *breakerScanner {
	s := newBreakerScanner(ammAddr, time.Second)
	s.chunk = chunk
	return s
}

func TestBreakerScannerSweepsHistoryInChunks(t *testing.T) {
	src := &fakeLogSource{
		head: 25,
		logs: []types.Log{
			breakerLog(3, 0, sigBreakerPaused, "0xaa"),
			breakerLog(22, 1, sigBreakerResumed, "0xbb"),
		},
	}
	s := newTestScanner(10)
	if err := s.refresh(context.Background(), src); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	// The whole range is covered, and no single request is wider than the chunk — that is
	// what keeps the query inside a node's eth_getLogs range cap.
	want := [][2]uint64{{0, 9}, {10, 19}, {20, 25}}
	if len(src.queries) != len(want) {
		t.Fatalf("queries = %v, want %v", src.queries, want)
	}
	for i, q := range src.queries {
		if q != want[i] {
			t.Fatalf("query %d = %v, want %v", i, q, want[i])
		}
	}

	hit, ok := s.latestAny()
	if !ok {
		t.Fatal("latestAny found nothing")
	}
	if got := hit.txHash.Hex(); got != common.HexToHash("0xbb").Hex() {
		t.Fatalf("latest tx = %s, want the block-22 event", got)
	}
}

func TestBreakerScannerOnlyReadsNewBlocks(t *testing.T) {
	src := &fakeLogSource{head: 12, logs: []types.Log{breakerLog(5, 0, sigBreakerPaused, "0xaa")}}
	s := newTestScanner(10)
	if err := s.refresh(context.Background(), src); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	first := len(src.queries)

	// Head has not moved: the poll must cost nothing beyond the block-number read. This is
	// the case that used to re-scan the entire chain four times a minute.
	if err := s.refresh(context.Background(), src); err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	if len(src.queries) != first {
		t.Fatalf("idle refresh issued %d extra queries, want 0", len(src.queries)-first)
	}

	// Head advances: only the new blocks are asked for, never from 0 again.
	src.head = 18
	src.logs = append(src.logs, breakerLog(17, 0, sigBreakerResumed, "0xcc"))
	if err := s.refresh(context.Background(), src); err != nil {
		t.Fatalf("third refresh: %v", err)
	}
	last := src.queries[len(src.queries)-1]
	if last != [2]uint64{13, 18} {
		t.Fatalf("incremental query = %v, want {13 18}", last)
	}
	hit, _ := s.latestAny()
	if hit.txHash.Hex() != common.HexToHash("0xcc").Hex() {
		t.Fatalf("latest tx = %s, want the newly appeared event", hit.txHash.Hex())
	}
}

func TestBreakerScannerKeepsProgressAcrossAFailedChunk(t *testing.T) {
	src := &fakeLogSource{
		head:     25,
		logs:     []types.Log{breakerLog(2, 0, sigBreakerPaused, "0xaa")},
		failFrom: 10,
	}
	s := newTestScanner(10)
	if err := s.refresh(context.Background(), src); err == nil {
		t.Fatal("refresh succeeded, want the chunk error surfaced")
	}
	if s.cursor != 9 {
		t.Fatalf("cursor = %d, want 9 — the blocks already read must not be re-read", s.cursor)
	}

	// The next poll resumes where the failure left off rather than starting over, so a long
	// first sweep is amortised across polls instead of failing from the top every time.
	src.failFrom = 0
	src.queries = nil
	if err := s.refresh(context.Background(), src); err != nil {
		t.Fatalf("resumed refresh: %v", err)
	}
	if src.queries[0] != [2]uint64{10, 19} {
		t.Fatalf("resumed at %v, want {10 19}", src.queries[0])
	}
}

func TestBreakerScannerTracksProposalSeparatelyFromLatestAction(t *testing.T) {
	proposalID := common.HexToHash("0xdeadbeef")
	src := &fakeLogSource{
		head: 8,
		logs: []types.Log{
			breakerLog(4, 0, sigBreakerPaused, "0xaa"),
			breakerLog(6, 0, sigResumeProposed, "0xbb", proposalID),
			breakerLog(6, 1, sigResumeSigned, "0xbb", proposalID),
			breakerLog(7, 0, sigResumeSigned, "0xcc", proposalID),
		},
	}
	s := newTestScanner(100)
	if err := s.refresh(context.Background(), src); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	// The citable action is the newest event of any kind — here the second signature.
	hit, ok := s.latestAny()
	if !ok || hit.txHash.Hex() != common.HexToHash("0xcc").Hex() {
		t.Fatalf("latestAny = %v, want the block-7 signature", hit.txHash.Hex())
	}
	// The proposal id comes from LogResumeProposed alone. A cache keyed on isPaused() would
	// have missed both signatures: proposeResume and a below-quorum signResume are whenPaused
	// and leave the flag set.
	proposed, ok := s.latestOf(sigResumeProposed)
	if !ok {
		t.Fatal("latestOf(proposed) found nothing")
	}
	if proposed.topics[1] != proposalID {
		t.Fatalf("proposal id = %s, want %s", proposed.topics[1].Hex(), proposalID.Hex())
	}
}

func TestBreakerScannerOrdersWithinABlockByLogIndex(t *testing.T) {
	src := &fakeLogSource{
		head: 3,
		logs: []types.Log{
			breakerLog(3, 5, sigResumeSigned, "0xbb"),
			breakerLog(3, 2, sigResumeProposed, "0xaa"),
		},
	}
	s := newTestScanner(100)
	if err := s.refresh(context.Background(), src); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	hit, _ := s.latestAny()
	if hit.txHash.Hex() != common.HexToHash("0xbb").Hex() {
		t.Fatalf("latest tx = %s, want the higher log index in the same block", hit.txHash.Hex())
	}
}

func TestBreakerScannerReportsAbsenceWithoutRescanning(t *testing.T) {
	src := &fakeLogSource{head: 30}
	s := newTestScanner(10)
	if err := s.refresh(context.Background(), src); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if _, ok := s.latestAny(); ok {
		t.Fatal("latestAny reported a hit on a pair with no breaker history")
	}
	swept := len(src.queries)

	// A pair that has never had a breaker action is the common case, and it must not pay for
	// a full sweep on every poll.
	if err := s.refresh(context.Background(), src); err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	if len(src.queries) != swept {
		t.Fatalf("re-scanned %d chunks after finding nothing, want 0", len(src.queries)-swept)
	}
}

// The scanner must survive the concurrent reads the per-pair poll fan-out produces.
func TestBreakerScannerConcurrentRefresh(t *testing.T) {
	src := &fakeLogSource{head: 50, logs: []types.Log{breakerLog(10, 0, sigBreakerPaused, "0xaa")}}
	s := newTestScanner(10)
	done := make(chan error, 4)
	for i := 0; i < 4; i++ {
		go func() {
			err := s.refresh(context.Background(), src)
			s.latestAny()
			done <- err
		}()
	}
	for i := 0; i < 4; i++ {
		if err := <-done; err != nil {
			t.Fatalf("concurrent refresh: %v", err)
		}
	}
	if s.cursor != 50 {
		t.Fatalf("cursor = %d, want 50", s.cursor)
	}
}

// A chunk boundary must not drop or double-count a log sitting exactly on it.
func TestBreakerScannerChunkBoundaryIsInclusive(t *testing.T) {
	src := &fakeLogSource{
		head: 20,
		logs: []types.Log{breakerLog(10, 0, sigBreakerPaused, "0xaa")},
	}
	s := newTestScanner(10)
	if err := s.refresh(context.Background(), src); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if _, ok := s.latestAny(); !ok {
		t.Fatal("log on the chunk boundary was missed")
	}
	// The ranges must tile the chain: contiguous, non-overlapping, ending at the head. An
	// off-by-one either way would silently skip a block or read one twice.
	var next uint64
	for _, q := range src.queries {
		if q[0] != next {
			t.Fatalf("range %v does not continue from %d", q, next)
		}
		next = q[1] + 1
	}
	if next != src.head+1 {
		t.Fatalf("ranges stopped at %d, want coverage through head %d", next-1, src.head)
	}
}

// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// A residue return has two failure points and only one used to be retried: an enqueued leg has
// a bridge position, so the relayer's queue drives it; a failed ENQUEUE has no position, so
// nothing ever tried again and the payer's unspent reserve stayed on the issuing CB's Hub
// address indefinitely. These tests pin the loop that closes that gap.

type fakeResidueRetryRepo struct {
	pending  []domain.CrossCurrencySwapOperation
	listErr  error
	statuses map[string]domain.ResidueReturnStatus
	posIDs   map[string]string
	attempts map[string]int
	nextAt   map[string]*time.Time
}

func newFakeResidueRetryRepo(ops ...domain.CrossCurrencySwapOperation) *fakeResidueRetryRepo {
	return &fakeResidueRetryRepo{
		pending:  ops,
		statuses: map[string]domain.ResidueReturnStatus{},
		posIDs:   map[string]string{},
		attempts: map[string]int{},
		nextAt:   map[string]*time.Time{},
	}
}

func (f *fakeResidueRetryRepo) ClaimRetryableResidues(context.Context, time.Time, int) ([]domain.CrossCurrencySwapOperation, error) {
	return f.pending, f.listErr
}

func (f *fakeResidueRetryRepo) UpdateResidue(_ context.Context, swapID, _, positionID string, status domain.ResidueReturnStatus) error {
	f.statuses[swapID] = status
	f.posIDs[swapID] = positionID
	return nil
}

func (f *fakeResidueRetryRepo) RecordResidueAttempt(_ context.Context, swapID string, attempts int, nextAttemptAt *time.Time) error {
	f.attempts[swapID] = attempts
	f.nextAt[swapID] = nextAttemptAt
	return nil
}

// retryResidueRelay stands in for the issuing CB's endpoint.
type retryResidueRelay struct {
	calls    int
	gotReq   CrossCurrencyResidueReturnRequest
	position string
	failWith error
}

func (r *retryResidueRelay) NotifyResidueReturn(_ context.Context, req CrossCurrencyResidueReturnRequest) (string, error) {
	r.calls++
	r.gotReq = req
	if r.failWith != nil {
		return "", r.failWith
	}
	return r.position, nil
}

func failedResidueSwap(swapID string, attempts int) domain.CrossCurrencySwapOperation {
	posID := "pos-bridge-in-" + swapID
	txHash := "0xswap-" + swapID
	return domain.CrossCurrencySwapOperation{
		SwapID:             swapID,
		CorrelationID:      "corr-" + swapID,
		PayerBankID:        "bank-itau",
		SourceCurrency:     "BRL",
		TargetCurrency:     "ARS",
		PoolPair:           "W-BRL-W-ARS",
		AmountIn:           "1000",
		AmountOut:          "900",
		MaxAmountIn:        "3000",
		Status:             domain.SwapStatusCompleted,
		BridgeInPositionID: &posID,
		SwapTxHash:         &txHash,
		ResidueAmount:      "2000",
		ResidueStatus:      domain.ResidueReturnFailed,
		ResidueAttempts:    attempts,
	}
}

// haltedBreaker reports a pool paused by governance.
type haltedBreaker struct{ halted bool }

func (h haltedBreaker) IsHalted(context.Context, string) (bool, error) { return h.halted, nil }

func retryOrchestrator(relay ResidueReturnRelayIface) *CrossCurrencySwapOrchestrator {
	return retryOrchestratorWithBreaker(relay, stubCBOK{})
}

func retryOrchestratorWithBreaker(relay ResidueReturnRelayIface, breaker CircuitBreakerChecker) *CrossCurrencySwapOrchestrator {
	return NewCrossCurrencySwapOrchestrator(
		stubSwapRepo{}, nil, &stubLockMint{}, stubBurnUnlock{},
		stubSwapService{}, stubPoolActive{}, breaker, nil, nil, nil,
	).WithResidueReturnRelay(relay).WithHubSignerAddress("0xCBHUB")
}

func TestRetryFailedResidueReturns_RecoversAndMarksEnqueued(t *testing.T) {
	repo := newFakeResidueRetryRepo(failedResidueSwap("swap-1", 0))
	relay := &retryResidueRelay{position: "residue-pos-1"}
	orch := retryOrchestrator(relay)

	attempted, recovered := orch.RetryFailedResidueReturns(context.Background(), repo, time.Now(), 10)

	if attempted != 1 || recovered != 1 {
		t.Fatalf("attempted=%d recovered=%d, want 1/1", attempted, recovered)
	}
	if relay.calls != 1 {
		t.Fatalf("expected one delegation to the issuing CB, got %d", relay.calls)
	}
	if repo.statuses["swap-1"] != domain.ResidueReturnEnqueued {
		t.Fatalf("status = %q, want RETURN_ENQUEUED", repo.statuses["swap-1"])
	}
	if repo.posIDs["swap-1"] != "residue-pos-1" {
		t.Fatalf("position not recorded: %q", repo.posIDs["swap-1"])
	}
	// A terminal outcome clears the schedule; leaving one would re-attempt a settled return.
	if repo.nextAt["swap-1"] != nil {
		t.Fatalf("expected no further attempt scheduled, got %v", repo.nextAt["swap-1"])
	}
	// The CB derives the amount itself, so the request carries the position and tx hash but no
	// amount at all.
	if relay.gotReq.BridgeInPositionID != "pos-bridge-in-swap-1" || relay.gotReq.SwapTxHash != "0xswap-swap-1" {
		t.Fatalf("request lacks the derivation inputs: %+v", relay.gotReq)
	}
	if relay.gotReq.PayerBankID != "bank-itau" || relay.gotReq.SpokeIn != "spoke-brl" {
		t.Fatalf("request lacks payer/spoke: %+v", relay.gotReq)
	}
}

// A duplicate is a success: the enqueue actually worked and only the response was lost, so the
// retry repairs a FALSE RETURN_FAILED instead of creating a second refund.
func TestRetryFailedResidueReturns_DuplicateRepairsTheRecord(t *testing.T) {
	repo := newFakeResidueRetryRepo(failedResidueSwap("swap-dup", 1))
	// The CB answers "duplicate" with the position that already refunded this swap; the relay
	// surfaces that as a normal success carrying that position id.
	relay := &retryResidueRelay{position: "residue-pos-existing"}
	orch := retryOrchestrator(relay)

	_, recovered := orch.RetryFailedResidueReturns(context.Background(), repo, time.Now(), 10)

	if recovered != 1 {
		t.Fatalf("a duplicate must be treated as recovered, got %d", recovered)
	}
	if repo.statuses["swap-dup"] != domain.ResidueReturnEnqueued {
		t.Fatalf("status = %q, want the record corrected to RETURN_ENQUEUED", repo.statuses["swap-dup"])
	}
	if repo.posIDs["swap-dup"] != "residue-pos-existing" {
		t.Fatalf("expected the existing position to be recorded, got %q", repo.posIDs["swap-dup"])
	}
}

func TestRetryFailedResidueReturns_SchedulesBackoffOnFailure(t *testing.T) {
	repo := newFakeResidueRetryRepo(failedResidueSwap("swap-2", 1))
	relay := &retryResidueRelay{failWith: errors.New("central bank unreachable")}
	orch := retryOrchestrator(relay)
	now := time.Now()

	_, recovered := orch.RetryFailedResidueReturns(context.Background(), repo, now, 10)

	if recovered != 0 {
		t.Fatalf("a failed attempt must not count as recovered")
	}
	if repo.attempts["swap-2"] != 2 {
		t.Fatalf("attempts = %d, want 2", repo.attempts["swap-2"])
	}
	// Still retryable — the status must NOT move to escalated before the ceiling.
	if s, ok := repo.statuses["swap-2"]; ok && s == domain.ResidueReturnEscalated {
		t.Fatalf("escalated too early")
	}
	next := repo.nextAt["swap-2"]
	if next == nil {
		t.Fatalf("expected the next attempt to be scheduled")
	}
	if !next.After(now) {
		t.Fatalf("next attempt %v is not after now %v", next, now)
	}
}

// Retrying forever would hide the problem. At the ceiling the value is still on the CB's Hub
// address, so the record must say a human is needed.
func TestRetryFailedResidueReturns_EscalatesAtTheCeiling(t *testing.T) {
	repo := newFakeResidueRetryRepo(failedResidueSwap("swap-3", residueMaxAttempts-1))
	relay := &retryResidueRelay{failWith: errors.New("still unreachable")}
	orch := retryOrchestrator(relay)

	orch.RetryFailedResidueReturns(context.Background(), repo, time.Now(), 10)

	if repo.statuses["swap-3"] != domain.ResidueReturnEscalated {
		t.Fatalf("status = %q, want RETURN_ESCALATED", repo.statuses["swap-3"])
	}
	if repo.attempts["swap-3"] != residueMaxAttempts {
		t.Fatalf("attempts = %d, want %d", repo.attempts["swap-3"], residueMaxAttempts)
	}
	if repo.nextAt["swap-3"] != nil {
		t.Fatalf("an escalated return must not stay scheduled")
	}
}

// Inputs a retry cannot fix are escalated instead of retried against a request the issuing CB
// could never authorise.
func TestRetryFailedResidueReturns_EscalatesUnfixableRows(t *testing.T) {
	noPosition := failedResidueSwap("swap-nopos", 0)
	noPosition.BridgeInPositionID = nil
	noTx := failedResidueSwap("swap-notx", 0)
	noTx.SwapTxHash = nil
	badAmount := failedResidueSwap("swap-badamt", 0)
	badAmount.ResidueAmount = "not-a-number"
	zeroAmount := failedResidueSwap("swap-zero", 0)
	zeroAmount.ResidueAmount = "0"

	for _, op := range []domain.CrossCurrencySwapOperation{noPosition, noTx, badAmount, zeroAmount} {
		t.Run(op.SwapID, func(t *testing.T) {
			repo := newFakeResidueRetryRepo(op)
			relay := &retryResidueRelay{position: "should-not-happen"}
			orch := retryOrchestrator(relay)

			orch.RetryFailedResidueReturns(context.Background(), repo, time.Now(), 10)

			if relay.calls != 0 {
				t.Fatalf("must not call the issuing CB with inputs it cannot authorise")
			}
			if repo.statuses[op.SwapID] != domain.ResidueReturnEscalated {
				t.Fatalf("status = %q, want RETURN_ESCALATED", repo.statuses[op.SwapID])
			}
			if repo.nextAt[op.SwapID] != nil {
				t.Fatalf("an escalated row must not stay scheduled")
			}
		})
	}
}

func TestRetryFailedResidueReturns_NoRepoOrEmptyQueueIsANoOp(t *testing.T) {
	orch := retryOrchestrator(&retryResidueRelay{})

	if a, r := orch.RetryFailedResidueReturns(context.Background(), nil, time.Now(), 10); a != 0 || r != 0 {
		t.Fatalf("nil repo must be a no-op, got %d/%d", a, r)
	}
	empty := newFakeResidueRetryRepo()
	if a, r := orch.RetryFailedResidueReturns(context.Background(), empty, time.Now(), 10); a != 0 || r != 0 {
		t.Fatalf("empty queue must be a no-op, got %d/%d", a, r)
	}
}

// A listing failure must not be mistaken for "nothing to do".
func TestRetryFailedResidueReturns_ListErrorIsNotSilentSuccess(t *testing.T) {
	repo := newFakeResidueRetryRepo(failedResidueSwap("swap-4", 0))
	repo.listErr = errors.New("db down")
	relay := &retryResidueRelay{}
	orch := retryOrchestrator(relay)

	attempted, recovered := orch.RetryFailedResidueReturns(context.Background(), repo, time.Now(), 10)

	if attempted != 0 || recovered != 0 {
		t.Fatalf("a list failure must attempt nothing, got %d/%d", attempted, recovered)
	}
	if relay.calls != 0 {
		t.Fatalf("must not call the CB when the queue could not be read")
	}
}

func TestResidueBackoff_GrowsAndIsCapped(t *testing.T) {
	if residueBackoff(1) >= residueBackoff(3) {
		t.Fatalf("backoff must grow with attempts")
	}
	if got := residueBackoff(99); got != residueBackoffCapSeconds*time.Second {
		t.Fatalf("backoff must cap at %ds, got %v", residueBackoffCapSeconds, got)
	}
}

// Pausing a pair by governance stops trading on it. The retry loop is the one unattended path
// that still moves value, so it must defer too — and deferring must not consume an attempt, or a
// long pause would exhaust the budget and escalate refunds that were never actually tried.
func TestRetryFailedResidueReturns_DefersWhileThePoolIsHalted(t *testing.T) {
	repo := newFakeResidueRetryRepo(failedResidueSwap("swap-halted", 2))
	relay := &retryResidueRelay{position: "should-not-happen"}
	orch := retryOrchestratorWithBreaker(relay, haltedBreaker{halted: true})
	now := time.Now()

	attempted, recovered := orch.RetryFailedResidueReturns(context.Background(), repo, now, 10)

	if relay.calls != 0 {
		t.Fatalf("must not dispatch a refund while the pool is halted")
	}
	if attempted != 0 || recovered != 0 {
		t.Fatalf("a deferral is not an attempt, got %d/%d", attempted, recovered)
	}
	if repo.attempts["swap-halted"] != 2 {
		t.Fatalf("attempts = %d, want the counter untouched at 2", repo.attempts["swap-halted"])
	}
	if next := repo.nextAt["swap-halted"]; next == nil || !next.After(now) {
		t.Fatalf("expected the row to be rescheduled for later, got %v", next)
	}
	if s, ok := repo.statuses["swap-halted"]; ok && s == domain.ResidueReturnEscalated {
		t.Fatalf("a halted pool must not escalate the row")
	}
}

// A 200 carrying no position is not a return in flight: the issuing CB answers that way when its
// own derivation finds nothing to give back, which contradicts the amount recorded here.
// Recording RETURN_ENQUEUED would close the case on a disagreement nobody ever sees.
func TestRetryFailedResidueReturns_EmptyPositionEscalatesInsteadOfClaimingSuccess(t *testing.T) {
	repo := newFakeResidueRetryRepo(failedResidueSwap("swap-nopos-resp", 0))
	relay := &retryResidueRelay{position: ""} // success, but nothing in flight
	orch := retryOrchestrator(relay)

	_, recovered := orch.RetryFailedResidueReturns(context.Background(), repo, time.Now(), 10)

	if recovered != 0 {
		t.Fatalf("an empty position must not count as recovered")
	}
	if repo.statuses["swap-nopos-resp"] != domain.ResidueReturnEscalated {
		t.Fatalf("status = %q, want RETURN_ESCALATED", repo.statuses["swap-nopos-resp"])
	}
	if repo.nextAt["swap-nopos-resp"] != nil {
		t.Fatalf("an escalated row must not stay scheduled")
	}
}

// A cancelled context means the process is going away. Counting it as a failed attempt would
// escalate rows sitting at the ceiling purely because a pod restarted.
func TestRetryFailedResidueReturns_CancelledContextDoesNotBurnAttempts(t *testing.T) {
	repo := newFakeResidueRetryRepo(
		failedResidueSwap("swap-c1", residueMaxAttempts-1),
		failedResidueSwap("swap-c2", residueMaxAttempts-1),
	)
	relay := &retryResidueRelay{position: "unused"}
	orch := retryOrchestrator(relay)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	attempted, recovered := orch.RetryFailedResidueReturns(ctx, repo, time.Now(), 10)

	if attempted != 0 || recovered != 0 {
		t.Fatalf("a cancelled sweep must attempt nothing, got %d/%d", attempted, recovered)
	}
	if relay.calls != 0 {
		t.Fatalf("must not dispatch on a cancelled context")
	}
	for _, id := range []string{"swap-c1", "swap-c2"} {
		if s, ok := repo.statuses[id]; ok && s == domain.ResidueReturnEscalated {
			t.Fatalf("%s escalated because of a restart, not a real failure", id)
		}
	}
}

// The backoff has to actually delay: below the sweep interval the schedule is meaningless and
// the whole attempt budget is spent one attempt per tick.
func TestResidueBackoff_ExceedsTheSweepInterval(t *testing.T) {
	const sweep = 60 * time.Second
	for attempts := 1; attempts <= residueMaxAttempts; attempts++ {
		if got := residueBackoff(attempts); got < sweep {
			t.Fatalf("backoff(%d) = %v, below the %v sweep interval — the schedule would never delay anything",
				attempts, got, sweep)
		}
	}
}

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
	claimed  int         // how many of pending have been handed out, one per claim
	claimAt  []time.Time // the timestamp each claim was given, in order
	listErr  error
	statuses map[string]domain.ResidueReturnStatus
	posIDs   map[string]string
	attempts map[string]int
	nextAt   map[string]*time.Time
	// deferredSince is when the current deferral window opened, and deferrals counts how many
	// times a row was put off. The window is what bounds an indefinite governance pause.
	deferredSince map[string]time.Time
	deferrals     map[string]int
}

func newFakeResidueRetryRepo(ops ...domain.CrossCurrencySwapOperation) *fakeResidueRetryRepo {
	f := &fakeResidueRetryRepo{
		pending:       ops,
		statuses:      map[string]domain.ResidueReturnStatus{},
		posIDs:        map[string]string{},
		attempts:      map[string]int{},
		nextAt:        map[string]*time.Time{},
		deferredSince: map[string]time.Time{},
		deferrals:     map[string]int{},
	}
	// These maps stand in for columns, so a seeded row's existing deferral window has to be
	// visible here too — otherwise the fake's "first stamp wins" has nothing to compare against
	// and it would overwrite a stamp the repository's COALESCE keeps.
	for _, op := range ops {
		if op.ResidueDeferredSince != nil {
			f.deferredSince[op.SwapID] = *op.ResidueDeferredSince
		}
	}
	return f
}

// ClaimNextRetryableResidue hands out one seeded row per call and then reports nothing due,
// mirroring the repository: a single row is claimed and leased immediately before its own
// dispatch, so the lease never has to cover more than one call to the issuing CB.
func (f *fakeResidueRetryRepo) ClaimNextRetryableResidue(_ context.Context, now time.Time) (*domain.CrossCurrencySwapOperation, error) {
	f.claimAt = append(f.claimAt, now)
	if f.listErr != nil {
		return nil, f.listErr
	}
	if f.claimed >= len(f.pending) {
		return nil, nil
	}
	op := f.pending[f.claimed]
	f.claimed++
	return &op, nil
}

func (f *fakeResidueRetryRepo) UpdateResidue(_ context.Context, swapID, _, positionID string, status domain.ResidueReturnStatus) error {
	f.statuses[swapID] = status
	f.posIDs[swapID] = positionID
	return nil
}

func (f *fakeResidueRetryRepo) RecordResidueAttempt(_ context.Context, swapID string, attempts int, nextAttemptAt *time.Time) error {
	f.attempts[swapID] = attempts
	f.nextAt[swapID] = nextAttemptAt
	// A real attempt ends any deferral window, exactly as the repository does.
	delete(f.deferredSince, swapID)
	return nil
}

func (f *fakeResidueRetryRepo) DeferResidue(_ context.Context, swapID string, nextAttemptAt, deferredSince time.Time) error {
	f.deferrals[swapID]++
	next := nextAttemptAt
	f.nextAt[swapID] = &next
	// First stamp wins, mirroring the COALESCE in the repository: the bound measures the whole
	// pause, not the time since the most recent sweep looked at it.
	if _, already := f.deferredSince[swapID]; !already {
		f.deferredSince[swapID] = deferredSince
	}
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
	if _, written := repo.attempts["swap-halted"]; written {
		t.Fatalf("a deferral must not write the attempt counter at all — it did, at %d", repo.attempts["swap-halted"])
	}
	if repo.deferrals["swap-halted"] != 1 {
		t.Fatalf("deferrals = %d, want 1", repo.deferrals["swap-halted"])
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

// The claim must be given a CURRENT timestamp, not the one the sweep started with.
//
// The repository writes the lease from the timestamp it is handed, so forwarding the sweep's
// own `now` would date every lease from the start of the sweep: by the time a long sweep
// reaches its later rows, their leases are already spent and another sweeper can claim them.
// That is the coupling between batch size and lease duration this design removes, reappearing
// through the clock instead of through the batch.
func TestRetryFailedResidueReturns_ClaimsWithACurrentTimestamp(t *testing.T) {
	repo := newFakeResidueRetryRepo(
		failedResidueSwap("swap-1", 0),
		failedResidueSwap("swap-2", 0),
	)
	orch := retryOrchestrator(&retryResidueRelay{position: "residue-pos"})

	// Deliberately stale: a sweep that began an hour ago. Any lease dated from here is already
	// expired, so a claim handed this timestamp protects nothing.
	sweepStart := time.Now().UTC().Add(-time.Hour)
	orch.RetryFailedResidueReturns(context.Background(), repo, sweepStart, 10)

	if len(repo.claimAt) == 0 {
		t.Fatal("no claim was made; this test cannot say anything about the timestamp")
	}
	for i, at := range repo.claimAt {
		if !at.After(sweepStart.Add(30 * time.Minute)) {
			t.Errorf("claim %d was given %s, dated from the sweep's start (%s) rather than now: "+
				"the lease it writes is already expired, so a long sweep hands its remaining "+
				"rows to another sweeper", i, at.UTC(), sweepStart.UTC())
		}
	}
}

// --- bounding an indefinite pause ---
//
// Deferring while a pair is halted is right, and not consuming an attempt for it is right, but
// together they had no end: the delay was computed from a counter that deferring never advances,
// so it never grew, and nothing ever escalated. A pair left paused — a governance decision that
// can outlast an incident by days — held the payer's unspent reserve on the issuing CB's Hub
// address forever, with a log line as the only trace. The refund is the one thing in this path
// that a pause must not silently swallow: the payer is over-debited until it lands.

func haltedSwapDeferredSince(swapID string, attempts int, since *time.Time) domain.CrossCurrencySwapOperation {
	op := failedResidueSwap(swapID, attempts)
	op.ResidueDeferredSince = since
	return op
}

func TestRetryFailedResidueReturns_StampsWhenTheDeferralBegan(t *testing.T) {
	repo := newFakeResidueRetryRepo(haltedSwapDeferredSince("swap-first-defer", 1, nil))
	relay := &retryResidueRelay{position: "should-not-happen"}
	orch := retryOrchestratorWithBreaker(relay, haltedBreaker{halted: true})
	now := time.Now()

	orch.RetryFailedResidueReturns(context.Background(), repo, now, 10)

	if relay.calls != 0 {
		t.Fatal("must not dispatch a refund while the pool is halted")
	}
	// Without a start timestamp there is nothing to measure the pause against, so the first
	// deferral has to record one.
	got, ok := repo.deferredSince["swap-first-defer"]
	if !ok {
		t.Fatal("the first deferral did not record when the pause started — the bound has nothing to measure")
	}
	if !got.Equal(now) {
		t.Fatalf("deferredSince = %v, want the sweep's now (%v)", got, now)
	}
	if s, ok := repo.statuses["swap-first-defer"]; ok && s == domain.ResidueReturnEscalated {
		t.Fatal("a pause that just started must not escalate")
	}
}

func TestRetryFailedResidueReturns_KeepsDeferringInsideTheBound(t *testing.T) {
	now := time.Now()
	// Well inside the bound: an incident-length pause is the case deferral exists for.
	since := now.Add(-1 * time.Hour)
	repo := newFakeResidueRetryRepo(haltedSwapDeferredSince("swap-recent-halt", 3, &since))
	relay := &retryResidueRelay{position: "should-not-happen"}
	orch := retryOrchestratorWithBreaker(relay, haltedBreaker{halted: true})

	attempted, _ := orch.RetryFailedResidueReturns(context.Background(), repo, now, 10)

	if attempted != 0 || relay.calls != 0 {
		t.Fatalf("attempted=%d relay.calls=%d — a halted pool inside the bound must still defer", attempted, relay.calls)
	}
	if s, ok := repo.statuses["swap-recent-halt"]; ok && s == domain.ResidueReturnEscalated {
		t.Fatal("escalated inside the bound — an hour-long pause is normal operation")
	}
	if next := repo.nextAt["swap-recent-halt"]; next == nil || !next.After(now) {
		t.Fatalf("expected the row rescheduled for later, got %v", next)
	}
	// The stamp must survive: measuring from the latest sweep instead of from the start of the
	// pause is what would make the bound unreachable.
	if got := repo.deferredSince["swap-recent-halt"]; !got.Equal(since) {
		t.Fatalf("deferredSince moved to %v, want the original %v", got, since)
	}
}

func TestRetryFailedResidueReturns_EscalatesAHaltThatOutlastsTheBound(t *testing.T) {
	now := time.Now()
	since := now.Add(-residueMaxDeferral() - time.Minute)
	repo := newFakeResidueRetryRepo(haltedSwapDeferredSince("swap-stuck-halt", 2, &since))
	relay := &retryResidueRelay{position: "should-not-happen"}
	orch := retryOrchestratorWithBreaker(relay, haltedBreaker{halted: true})

	attempted, recovered := orch.RetryFailedResidueReturns(context.Background(), repo, now, 10)

	if relay.calls != 0 {
		t.Fatal("escalating must not dispatch — the pair is still halted")
	}
	if attempted != 0 || recovered != 0 {
		t.Fatalf("attempted=%d recovered=%d — escalating a deferral is not an attempt", attempted, recovered)
	}
	if repo.statuses["swap-stuck-halt"] != domain.ResidueReturnEscalated {
		t.Fatalf("status = %q, want RETURN_ESCALATED: a pause this long needs a human, not another deferral",
			repo.statuses["swap-stuck-halt"])
	}
	// Terminal for automation: leaving a schedule behind would keep the row in the retryable set.
	if next := repo.nextAt["swap-stuck-halt"]; next != nil {
		t.Fatalf("an escalated row must not stay scheduled, got %v", next)
	}
	// No attempt was ever made against this row, so the counter must not claim one.
	if got := repo.attempts["swap-stuck-halt"]; got != 2 {
		t.Fatalf("attempts = %d, want 2 — escalating a pause must not invent an attempt", got)
	}
}

func TestResidueMaxDeferral_SitsAboveTheBackoffCap(t *testing.T) {
	// If the bound were near the backoff cap, the first or second deferral would already exceed
	// it and a routine pause would escalate every pending refund.
	if residueMaxDeferral() <= residueBackoff(99) {
		t.Fatalf("bound %v must sit well above the backoff cap %v", residueMaxDeferral(), residueBackoff(99))
	}
}

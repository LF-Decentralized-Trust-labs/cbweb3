// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// A cross-currency delivery has two failure points and only one used to be retried: an accepted
// notification creates a bridge position on CB-B, so that CB's relayer queue drives it; a
// REJECTED notification creates no position anywhere, the Cacti relay forwards exactly once, and
// nothing ever tried again — the swapped value stayed on the Hub with no beneficiary while the
// payer stayed debited. That is the partial settlement the constitution forbids, and these tests
// pin the loop that closes it.

type fakeBridgeOutRetryRepo struct {
	pending  []domain.CrossCurrencySwapOperation
	claimed  int
	claimAt  []time.Time
	claimErr error

	statuses map[string]domain.BridgeOutDeliveryStatus
	attempts map[string]int
	nextAt   map[string]*time.Time
	posIDs   map[string]string

	deferredSince map[string]time.Time
	deferrals     map[string]int
	// reasons captures what a person would read on the swap record after a recovery.
	reasons map[string]string
}

func newFakeBridgeOutRetryRepo(ops ...domain.CrossCurrencySwapOperation) *fakeBridgeOutRetryRepo {
	f := &fakeBridgeOutRetryRepo{
		pending:       ops,
		statuses:      map[string]domain.BridgeOutDeliveryStatus{},
		attempts:      map[string]int{},
		nextAt:        map[string]*time.Time{},
		posIDs:        map[string]string{},
		deferredSince: map[string]time.Time{},
		deferrals:     map[string]int{},
		reasons:       map[string]string{},
	}
	// These maps stand in for columns, so a seeded row's existing deferral window has to be
	// visible here too — otherwise "first stamp wins" has nothing to compare against and the
	// fake would overwrite a stamp the repository's COALESCE keeps.
	for _, op := range ops {
		if op.BridgeOutDeferredSince != nil {
			f.deferredSince[op.SwapID] = *op.BridgeOutDeferredSince
		}
	}
	return f
}

// ClaimNextRetryableBridgeOut hands out one seeded row per call then reports nothing due,
// mirroring the repository: one row is claimed and leased immediately before its own dispatch,
// so the lease never has to cover more than one call to the relay.
func (f *fakeBridgeOutRetryRepo) ClaimNextRetryableBridgeOut(_ context.Context, now time.Time) (*domain.CrossCurrencySwapOperation, error) {
	f.claimAt = append(f.claimAt, now)
	if f.claimErr != nil {
		return nil, f.claimErr
	}
	if f.claimed >= len(f.pending) {
		return nil, nil
	}
	op := f.pending[f.claimed]
	f.claimed++
	return &op, nil
}

func (f *fakeBridgeOutRetryRepo) UpdateBridgeOutDelivery(_ context.Context, swapID string, status domain.BridgeOutDeliveryStatus, attempts int, nextAttemptAt *time.Time) error {
	f.statuses[swapID] = status
	f.attempts[swapID] = attempts
	f.nextAt[swapID] = nextAttemptAt
	// Recording an attempt clears the deferral window, exactly as the column write does.
	delete(f.deferredSince, swapID)
	return nil
}

func (f *fakeBridgeOutRetryRepo) DeferBridgeOut(_ context.Context, swapID string, nextAttemptAt, deferredSince time.Time) error {
	f.nextAt[swapID] = &nextAttemptAt
	f.deferrals[swapID]++
	// First stamp wins, mirroring COALESCE.
	if _, open := f.deferredSince[swapID]; !open {
		f.deferredSince[swapID] = deferredSince
	}
	return nil
}

func (f *fakeBridgeOutRetryRepo) UpdateFailureReason(_ context.Context, swapID string, reason string) error {
	f.reasons[swapID] = reason
	return nil
}

func (f *fakeBridgeOutRetryRepo) UpdateBridgeOutPositionID(_ context.Context, swapID string, positionID string) error {
	f.posIDs[swapID] = positionID
	return nil
}

// recordingRelay counts NotifyBridgeOut calls and can be made to fail a chosen number of times
// before succeeding, which is how "recovers on the third attempt" is expressed.
type recordingRelay struct {
	calls     int
	failFirst int
	reqs      []CactiCrossCurrencyBridgeOutRequest
	echo      string
}

func (r *recordingRelay) NotifyBridgeOut(_ context.Context, req CactiCrossCurrencyBridgeOutRequest) (string, error) {
	r.calls++
	r.reqs = append(r.reqs, req)
	if r.calls <= r.failFirst {
		return "", errors.New("HTTP 502 from relay: beneficiary bank not found or not active")
	}
	if r.echo == "" {
		return "echo-" + req.CorrelationID, nil
	}
	return r.echo, nil
}

// haltedPairs reports a fixed set of pairs as halted by governance.
type haltedPairs map[string]bool

func (h haltedPairs) IsHalted(_ context.Context, pair string) (bool, error) { return h[pair], nil }

func failedDelivery(swapID string, attempts int) domain.CrossCurrencySwapOperation {
	hash := "0xswap" + swapID
	return domain.CrossCurrencySwapOperation{
		SwapID:            swapID,
		CorrelationID:     "corr-" + swapID,
		PayerBankID:       "bank-itau",
		BeneficiaryBankID: "bank-macro",
		SourceCurrency:    "BRL",
		TargetCurrency:    "ARS",
		PoolPair:          "W-BRL-ARS",
		AmountOut:         "5",
		SwapTxHash:        &hash,
		BridgeOutStatus:   domain.BridgeOutDeliveryFailed,
		BridgeOutAttempts: attempts,
	}
}

func orchestratorForRetry(relay *recordingRelay) *CrossCurrencySwapOrchestrator {
	return &CrossCurrencySwapOrchestrator{cactiRelay: relay}
}

// TestRetryFailedBridgeOuts_RecoversTheReportedCase is the scenario from the incident: the
// beneficiary bank was KYC_APPROVED rather than ACTIVE, so CB-B rejected the delivery. Once the
// bank finishes onboarding, the same notification succeeds — and nothing used to send it again.
func TestRetryFailedBridgeOuts_RecoversTheReportedCase(t *testing.T) {
	relay := &recordingRelay{}
	repo := newFakeBridgeOutRetryRepo(failedDelivery("s1", 1))
	o := orchestratorForRetry(relay)

	attempted, recovered := o.RetryFailedBridgeOuts(context.Background(), repo, time.Now(), 10)

	if attempted != 1 || recovered != 1 {
		t.Fatalf("attempted=%d recovered=%d, want 1 and 1", attempted, recovered)
	}
	if got := repo.statuses["s1"]; got != domain.BridgeOutDeliveryNotified {
		t.Errorf("status = %q, want %q", got, domain.BridgeOutDeliveryNotified)
	}
	if repo.nextAt["s1"] != nil {
		t.Error("a delivered swap must not stay scheduled for another attempt")
	}
	if repo.posIDs["s1"] == "" {
		t.Error("the correlation CB-B echoes back must be persisted, or the delivery cannot be traced")
	}
	// The swap's own status stays FAILED on purpose — the payer was already shown that
	// verdict. So this field is the only place a person can see that the beneficiary was
	// eventually paid, and it has to say so, with how many attempts it took.
	reason := repo.reasons["s1"]
	if reason == "" {
		t.Fatal("nothing recorded on the swap record: it would still read as a plain failure " +
			"after the beneficiary was paid")
	}
	if !strings.Contains(reason, "recovered on attempt 2") {
		t.Errorf("the recovery must name the attempt it took, got %q", reason)
	}
	if !strings.Contains(reason, "beneficiary has been paid") {
		t.Errorf("the recovery must say the beneficiary was paid, got %q", reason)
	}
}

// TestRetryFailedBridgeOuts_RebuildsThePayloadFromTheRow proves the retry needs no stored
// payload. Every field is a column or re-derived — which is why this fix costs four columns and
// not a serialized request blob.
func TestRetryFailedBridgeOuts_RebuildsThePayloadFromTheRow(t *testing.T) {
	relay := &recordingRelay{}
	repo := newFakeBridgeOutRetryRepo(failedDelivery("s1", 1))

	orchestratorForRetry(relay).RetryFailedBridgeOuts(context.Background(), repo, time.Now(), 10)

	if len(relay.reqs) != 1 {
		t.Fatalf("relay calls = %d, want 1", len(relay.reqs))
	}
	req := relay.reqs[0]
	if req.CorrelationID != "corr-s1" || req.SwapTxHash != "0xswaps1" || req.PoolPair != "W-BRL-ARS" ||
		req.AmountOut != "5" || req.BeneficiaryBankID != "bank-macro" {
		t.Errorf("payload not rebuilt from the row: %+v", req)
	}
	if req.SpokeOut != "spoke-ars" {
		t.Errorf("SpokeOut = %q, want spoke-ars derived from the target currency", req.SpokeOut)
	}
	// Deliberately absent: CB-B ignores it and reads the burn-from address off the receipt, so
	// sending a stored copy would be dead weight that could go stale.
	if req.SwapSenderAddress != "" {
		t.Errorf("SwapSenderAddress = %q, want empty — CB-B re-derives it from the swap receipt", req.SwapSenderAddress)
	}
}

// TestRetryFailedBridgeOuts_SchedulesBackoffAndEscalates covers the budget. Attempts below the
// ceiling reschedule; the last one gives up and says so, because past that point only a human
// can choose between a late delivery and a refund.
func TestRetryFailedBridgeOuts_SchedulesBackoffAndEscalates(t *testing.T) {
	now := time.Now()

	t.Run("below the ceiling it reschedules", func(t *testing.T) {
		relay := &recordingRelay{failFirst: 1}
		repo := newFakeBridgeOutRetryRepo(failedDelivery("s1", 1))

		orchestratorForRetry(relay).RetryFailedBridgeOuts(context.Background(), repo, now, 10)

		if got := repo.statuses["s1"]; got != domain.BridgeOutDeliveryFailed {
			t.Errorf("status = %q, want %q", got, domain.BridgeOutDeliveryFailed)
		}
		if repo.attempts["s1"] != 2 {
			t.Errorf("attempts = %d, want 2", repo.attempts["s1"])
		}
		next := repo.nextAt["s1"]
		if next == nil {
			t.Fatal("a retryable failure must be rescheduled, or nothing picks it up again")
		}
		if !next.After(now) {
			t.Errorf("next attempt %s is not after now %s", next, now)
		}
	})

	t.Run("at the ceiling it escalates", func(t *testing.T) {
		relay := &recordingRelay{failFirst: 1}
		repo := newFakeBridgeOutRetryRepo(failedDelivery("s1", bridgeOutMaxAttempts-1))

		orchestratorForRetry(relay).RetryFailedBridgeOuts(context.Background(), repo, now, 10)

		if got := repo.statuses["s1"]; got != domain.BridgeOutDeliveryEscalated {
			t.Errorf("status = %q, want %q", got, domain.BridgeOutDeliveryEscalated)
		}
		if repo.nextAt["s1"] != nil {
			t.Error("an escalated delivery must not stay scheduled — it needs a human, not another sweep")
		}
	})
}

// TestRetryFailedBridgeOuts_DefersWhileThePairIsHalted: governance pausing a pair stops trading
// on it, and delivering against that pair anyway would leave the one unattended path still
// moving value. Deferring must not consume an attempt, or a pause would silently spend the whole
// budget.
func TestRetryFailedBridgeOuts_DefersWhileThePairIsHalted(t *testing.T) {
	now := time.Now()
	relay := &recordingRelay{}
	repo := newFakeBridgeOutRetryRepo(failedDelivery("s1", 1))
	o := orchestratorForRetry(relay)
	o.circuitBreakerCheck = haltedPairs{"W-BRL-ARS": true}

	attempted, recovered := o.RetryFailedBridgeOuts(context.Background(), repo, now, 10)

	if attempted != 0 || recovered != 0 {
		t.Errorf("attempted=%d recovered=%d, want 0 and 0 — a deferral is not an attempt", attempted, recovered)
	}
	if relay.calls != 0 {
		t.Errorf("relay was called %d times while the pair was halted", relay.calls)
	}
	if repo.deferrals["s1"] != 1 {
		t.Errorf("deferrals = %d, want 1", repo.deferrals["s1"])
	}
	if _, open := repo.deferredSince["s1"]; !open {
		t.Error("the deferral window must be opened, or an indefinite pause has no bound")
	}
	if repo.attempts["s1"] != 0 {
		t.Errorf("attempts recorded = %d, want 0 — deferring must not consume the budget", repo.attempts["s1"])
	}
}

// TestRetryFailedBridgeOuts_EscalatesAPauseThatOutlivesTheBound is the other end of that:
// deferring does not consume an attempt, so the ceiling cannot end the wait. Without a bound of
// its own an indefinitely paused pair strands the beneficiary forever.
func TestRetryFailedBridgeOuts_EscalatesAPauseThatOutlivesTheBound(t *testing.T) {
	now := time.Now()
	opened := now.Add(-bridgeOutMaxDeferral() - time.Minute)
	op := failedDelivery("s1", 1)
	op.BridgeOutDeferredSince = &opened

	relay := &recordingRelay{}
	repo := newFakeBridgeOutRetryRepo(op)
	o := orchestratorForRetry(relay)
	o.circuitBreakerCheck = haltedPairs{"W-BRL-ARS": true}

	o.RetryFailedBridgeOuts(context.Background(), repo, now, 10)

	if got := repo.statuses["s1"]; got != domain.BridgeOutDeliveryEscalated {
		t.Errorf("status = %q, want %q", got, domain.BridgeOutDeliveryEscalated)
	}
	if repo.attempts["s1"] != 1 {
		t.Errorf("attempts = %d, want 1 unchanged — no attempt was made, and claiming one would "+
			"misreport why the row was given up on", repo.attempts["s1"])
	}
}

// TestRetryFailedBridgeOuts_EscalatesWithoutASwapHash: without the hash CB-B can verify nothing,
// so no retry can ever succeed. Escalate instead of burning the whole budget failing identically.
func TestRetryFailedBridgeOuts_EscalatesWithoutASwapHash(t *testing.T) {
	op := failedDelivery("s1", 1)
	op.SwapTxHash = nil

	relay := &recordingRelay{}
	repo := newFakeBridgeOutRetryRepo(op)

	orchestratorForRetry(relay).RetryFailedBridgeOuts(context.Background(), repo, time.Now(), 10)

	if relay.calls != 0 {
		t.Errorf("relay was called %d times for a row that can never succeed", relay.calls)
	}
	if got := repo.statuses["s1"]; got != domain.BridgeOutDeliveryEscalated {
		t.Errorf("status = %q, want %q", got, domain.BridgeOutDeliveryEscalated)
	}
}

// TestRetryFailedBridgeOuts_StopsOnACancelledContext: a cancelled context means the process is
// going away, not that the attempt failed. Counting it would burn attempts — and escalate rows
// sitting at the ceiling — for no reason other than a restart.
func TestRetryFailedBridgeOuts_StopsOnACancelledContext(t *testing.T) {
	relay := &recordingRelay{}
	repo := newFakeBridgeOutRetryRepo(failedDelivery("s1", 1), failedDelivery("s2", 1))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	attempted, _ := orchestratorForRetry(relay).RetryFailedBridgeOuts(ctx, repo, time.Now(), 10)

	if attempted != 0 {
		t.Errorf("attempted = %d, want 0 on a cancelled context", attempted)
	}
	if relay.calls != 0 {
		t.Errorf("relay was called %d times during shutdown", relay.calls)
	}
	if len(repo.statuses) != 0 {
		t.Errorf("a shutdown must not record any outcome, got %v", repo.statuses)
	}
}

// TestRetryFailedBridgeOuts_NoRelayIsANoOp: a gateway with no relay configured is not the
// sovereign path, and sweeping there would claim rows it can never dispatch — burning nothing
// but hiding them behind a lease.
func TestRetryFailedBridgeOuts_NoRelayIsANoOp(t *testing.T) {
	repo := newFakeBridgeOutRetryRepo(failedDelivery("s1", 1))
	o := &CrossCurrencySwapOrchestrator{}

	attempted, recovered := o.RetryFailedBridgeOuts(context.Background(), repo, time.Now(), 10)

	if attempted != 0 || recovered != 0 {
		t.Errorf("attempted=%d recovered=%d, want 0 and 0", attempted, recovered)
	}
	if len(repo.claimAt) != 0 {
		t.Error("no row must be claimed when there is no relay to dispatch to")
	}
}

// TestBridgeOutBackoff_DoublesAndIsCapped pins the schedule the comments describe, so the
// numbers cannot drift from the reasoning written beside them.
func TestBridgeOutBackoff_DoublesAndIsCapped(t *testing.T) {
	for attempts, want := range map[int]time.Duration{
		1: 60 * time.Second,
		2: 120 * time.Second,
		3: 240 * time.Second,
		4: 480 * time.Second,
		5: 960 * time.Second,
	} {
		if got := bridgeOutBackoff(attempts); got != want {
			t.Errorf("bridgeOutBackoff(%d) = %s, want %s", attempts, got, want)
		}
	}
	// The cap must sit above every delay the ceiling can actually schedule, or the doubling
	// would flatten inside the working range and the later attempts would bunch up.
	if bridgeOutBackoff(bridgeOutMaxAttempts) >= bridgeOutBackoffCapSeconds*time.Second {
		t.Errorf("the cap (%ds) binds within the attempt budget — the schedule the comments "+
			"describe is not the schedule that runs", bridgeOutBackoffCapSeconds)
	}
	// And the deferral bound must sit above the longest schedulable delay, or a routine
	// incident-length pause would escalate every pending delivery instead of waiting it out.
	if bridgeOutMaxDeferral() <= bridgeOutBackoff(bridgeOutMaxAttempts) {
		t.Error("the deferral bound is not above the longest retry delay — a normal pause would escalate")
	}
}

// capturingSwapRepo records what the orchestrator wrote about the delivery leg. It exists
// because the marking is the half the retry loop cannot test: the loop starts from a row that
// is ALREADY DELIVERY_FAILED, so nothing above proves a real failure ever reaches that state.
type capturingSwapRepo struct {
	stubSwapRepo
	status   domain.BridgeOutDeliveryStatus
	attempts int
	nextAt   *time.Time
	calls    int
}

func (r *capturingSwapRepo) UpdateBridgeOutDelivery(_ context.Context, _ string, status domain.BridgeOutDeliveryStatus, attempts int, nextAttemptAt *time.Time) error {
	r.calls++
	r.status = status
	r.attempts = attempts
	r.nextAt = nextAttemptAt
	return nil
}

// rejectingCactiRelay rejects the delivery the way CB-B did in the incident: the beneficiary bank
// was KYC_APPROVED rather than ACTIVE, so the notification came back 422 through a relay 502.
type rejectingCactiRelay struct{ calls int }

func (f *rejectingCactiRelay) NotifyBridgeOut(_ context.Context, _ CactiCrossCurrencyBridgeOutRequest) (string, error) {
	f.calls++
	return "", errors.New(`cacti returned HTTP 502: HTTP 422 {"code":"BENEFICIARY_NOT_FOUND"}`)
}

// TestExecute_MarksAFailedDeliveryRetryable is the assertion the retry loop depends on and
// cannot make for itself.
//
// Before this, a rejected delivery only set the swap to FAILED and logged that a human was
// needed. The sweeper looks for DELIVERY_FAILED, so without this write it would never see the
// row: the value would sit on the Hub with no beneficiary exactly as it did in the incident,
// and every test above would still pass because they start from a row already in that state.
func TestExecute_MarksAFailedDeliveryRetryable(t *testing.T) {
	repo := &capturingSwapRepo{}
	relay := &rejectingCactiRelay{}

	orch := NewCrossCurrencySwapOrchestrator(
		repo,
		nil, // quoteRepo
		&stubLockMint{},
		stubBurnUnlock{},
		&recordingSwapService{},
		stubPoolActive{},
		stubCBOK{},
		nil, // rollbackCoordinator
		nil, // bridgeAssets
		nil, // bridgePoller
	).WithBridgeInRelay(&stubBridgeInRelay{}).
		WithHubSwapRelay(&stubHubSwapRelay{result: &SwapResult{
			TxHash: "0xswap", AmountIn: "800", HubSenderAddress: "0xCBHUB",
		}}).
		WithCactiRelay(relay)

	before := time.Now()
	_, err := orch.Execute(context.Background(), hubSwapRequest())
	if err == nil {
		t.Fatal("a rejected delivery must still fail the caller's swap — the beneficiary got nothing")
	}
	if relay.calls != 1 {
		t.Fatalf("relay calls = %d, want 1", relay.calls)
	}

	if repo.status != domain.BridgeOutDeliveryFailed {
		t.Errorf("delivery status = %q, want %q — without it the sweeper never finds this row",
			repo.status, domain.BridgeOutDeliveryFailed)
	}
	if repo.attempts != 1 {
		t.Errorf("attempts = %d, want 1 — the first failure IS the first attempt", repo.attempts)
	}
	if repo.nextAt == nil {
		t.Fatal("no next attempt scheduled — the row would sit DELIVERY_FAILED forever with nothing due")
	}
	if !repo.nextAt.After(before) {
		t.Errorf("next attempt %s is not in the future", repo.nextAt)
	}
	// The first retry must not be immediate: the dominant cause is a person finishing an
	// onboarding, and hammering the relay would spend the budget before that can happen.
	if repo.nextAt.Sub(before) < bridgeOutBackoffBaseSeconds*time.Second/2 {
		t.Errorf("first retry scheduled %s out, too soon for the human cause this recovers from",
			repo.nextAt.Sub(before))
	}
}

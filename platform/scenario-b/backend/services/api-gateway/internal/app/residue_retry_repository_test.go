// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"gorm.io/gorm"
)

// The retry queue is a query, and the predicate is where this can go wrong: picking up a leg the
// relayer already owns would double-refund, and skipping a NULL schedule would never retry the
// very first failure — which is the state a failed enqueue leaves behind.

func residueOp(swapID string, status domain.ResidueReturnStatus, createdAt time.Time, nextAt *time.Time) *domain.CrossCurrencySwapOperation {
	posID := "pos-" + swapID
	tx := "0x" + swapID
	return &domain.CrossCurrencySwapOperation{
		SwapID:               swapID,
		CorrelationID:        "corr-" + swapID,
		PayerBankID:          "bank-itau",
		BeneficiaryBankID:    "bank-macro",
		SourceCurrency:       "BRL",
		TargetCurrency:       "ARS",
		PoolPair:             "W-BRL-W-ARS",
		AmountIn:             "1000",
		AmountOut:            "900",
		MaxAmountIn:          "3000",
		Status:               domain.SwapStatusCompleted,
		BridgeInPositionID:   &posID,
		SwapTxHash:           &tx,
		ResidueAmount:        "2000",
		ResidueStatus:        status,
		ResidueNextAttemptAt: nextAt,
		CreatedAt:            createdAt,
	}
}

func TestClaimRetryableResidues_SelectsOnlyFailedAndDue(t *testing.T) {
	db := repoDB(t, &domain.CrossCurrencySwapOperation{})
	r := newCrossCurrencySwapRepository(db)
	ctx := context.Background()
	now := time.Now().UTC()
	past := now.Add(-time.Minute)
	future := now.Add(time.Hour)
	base := now.Add(-24 * time.Hour)

	rows := []*domain.CrossCurrencySwapOperation{
		// Due: a first failure has no schedule yet, which must count as due immediately.
		residueOp("failed-nullsched", domain.ResidueReturnFailed, base, nil),
		// Due: its backoff has elapsed.
		residueOp("failed-due", domain.ResidueReturnFailed, base.Add(time.Second), &past),
		// Not due: still inside its backoff.
		residueOp("failed-future", domain.ResidueReturnFailed, base, &future),
		// Not ours: the relayer's own queue drives an enqueued leg. Retrying it here would ask
		// the CB for a second refund of the same swap.
		residueOp("enqueued", domain.ResidueReturnEnqueued, base, nil),
		// Nothing to return.
		residueOp("none", domain.ResidueNone, base, nil),
		// Gave up: needs a human, not another attempt.
		residueOp("escalated", domain.ResidueReturnEscalated, base, nil),
		// Legacy rows predate the field entirely.
		residueOp("legacy-empty", "", base, nil),
	}
	for _, op := range rows {
		if err := db.Create(op).Error; err != nil {
			t.Fatalf("seed %s: %v", op.SwapID, err)
		}
	}

	got, err := r.ClaimRetryableResidues(ctx, now, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	ids := make([]string, 0, len(got))
	for i := range got {
		ids = append(ids, got[i].SwapID)
	}
	want := []string{"failed-nullsched", "failed-due"} // oldest first
	if len(ids) != len(want) {
		t.Fatalf("selected %v, want exactly %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("selected %v, want %v (oldest first)", ids, want)
		}
	}
}

func TestClaimRetryableResidues_RespectsLimit(t *testing.T) {
	db := repoDB(t, &domain.CrossCurrencySwapOperation{})
	r := newCrossCurrencySwapRepository(db)
	now := time.Now().UTC()

	for i, id := range []string{"a", "b", "c"} {
		op := residueOp("failed-"+id, domain.ResidueReturnFailed, now.Add(time.Duration(i)*time.Second), nil)
		if err := db.Create(op).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	got, err := r.ClaimRetryableResidues(context.Background(), now.Add(time.Hour), 2)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected the batch to be bounded at 2, got %d", len(got))
	}
	// A non-positive limit must not degrade into an unbounded scan. Proving that needs MORE rows
	// than the default bound: asserting against 3 rows would pass whether the default is 50,
	// 1000, or absent entirely.
	for i := 0; i < 60; i++ {
		op := residueOp("failed-bulk-"+strconv.Itoa(i), domain.ResidueReturnFailed,
			now.Add(time.Duration(i+10)*time.Second), nil)
		if err := db.Create(op).Error; err != nil {
			t.Fatalf("seed bulk: %v", err)
		}
	}
	all, err := r.ClaimRetryableResidues(context.Background(), now.Add(time.Hour), 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 50 {
		t.Fatalf("expected the default bound of 50, got %d", len(all))
	}
}

// The ceiling is enforced by the query as well as by the status write. Those are two separate
// statements: if the escalation write fails while the counter write lands, the row stays
// RETURN_FAILED at the ceiling, and without this bound it would be re-dispatched to the issuing
// CB on every sweep forever.
func TestClaimRetryableResidues_ExcludesRowsAtTheAttemptCeiling(t *testing.T) {
	db := repoDB(t, &domain.CrossCurrencySwapOperation{})
	r := newCrossCurrencySwapRepository(db)
	now := time.Now().UTC()

	below := residueOp("below-ceiling", domain.ResidueReturnFailed, now, nil)
	below.ResidueAttempts = services.ResidueMaxAttempts() - 1
	atCeiling := residueOp("at-ceiling", domain.ResidueReturnFailed, now, nil)
	atCeiling.ResidueAttempts = services.ResidueMaxAttempts()
	for _, op := range []*domain.CrossCurrencySwapOperation{below, atCeiling} {
		if err := db.Create(op).Error; err != nil {
			t.Fatalf("seed %s: %v", op.SwapID, err)
		}
	}

	got, err := r.ClaimRetryableResidues(context.Background(), now.Add(time.Hour), 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].SwapID != "below-ceiling" {
		ids := make([]string, 0, len(got))
		for i := range got {
			ids = append(ids, got[i].SwapID)
		}
		t.Fatalf("selected %v, want only below-ceiling", ids)
	}
}

func TestRecordResidueAttempt_PersistsAndClearsTheSchedule(t *testing.T) {
	db := repoDB(t, &domain.CrossCurrencySwapOperation{})
	r := newCrossCurrencySwapRepository(db)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := db.Create(residueOp("swap-1", domain.ResidueReturnFailed, now, nil)).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	next := now.Add(8 * time.Second)
	if err := r.RecordResidueAttempt(ctx, "swap-1", 3, &next); err != nil {
		t.Fatalf("record: %v", err)
	}
	// Each assertion reloads into a FRESH struct: GORM's scan does not overwrite an
	// already-populated pointer field with NULL, so reusing one would report a cleared column
	// as still set.
	scheduled := reloadSwap(t, db, "swap-1")
	if scheduled.ResidueAttempts != 3 {
		t.Fatalf("attempts = %d, want 3", scheduled.ResidueAttempts)
	}
	if scheduled.ResidueNextAttemptAt == nil {
		t.Fatalf("expected the next attempt to be persisted")
	}

	// A terminal outcome clears the schedule; leaving one would re-attempt a settled return.
	if err := r.RecordResidueAttempt(ctx, "swap-1", 4, nil); err != nil {
		t.Fatalf("record terminal: %v", err)
	}
	cleared := reloadSwap(t, db, "swap-1")
	if cleared.ResidueNextAttemptAt != nil {
		t.Fatalf("expected the schedule to be cleared, got %v", cleared.ResidueNextAttemptAt)
	}
	if cleared.ResidueAttempts != 4 {
		t.Fatalf("attempts = %d, want 4", cleared.ResidueAttempts)
	}
}

// reloadSwap reads a swap into a fresh struct.
func reloadSwap(t *testing.T, db *gorm.DB, swapID string) domain.CrossCurrencySwapOperation {
	t.Helper()
	var got domain.CrossCurrencySwapOperation
	if err := db.Where("swap_id = ?", swapID).First(&got).Error; err != nil {
		t.Fatalf("reload %s: %v", swapID, err)
	}
	return got
}

// The claim is the point of this query, not a side effect. Two gateway replicas sweep on their
// own timers; a plain SELECT hands both the same rows and both dispatch to the issuing CB. That
// does not double-refund — the endpoint is idempotent on (swap_tx_hash, RESIDUE) — but it burns
// two attempts against one row's ceiling and doubles the load for nothing.
//
// SQLite cannot exercise FOR UPDATE SKIP LOCKED (single writer, and it rejects the clause, so it
// is Postgres-only). What IS portable, and is what actually keeps the rows apart after the claim
// transaction commits, is the lease — so that is what these pin.
func TestClaimRetryableResidues_LeasesTheRowsItReturns(t *testing.T) {
	db := repoDB(t, &domain.CrossCurrencySwapOperation{})
	r := newCrossCurrencySwapRepository(db)
	ctx := context.Background()
	now := time.Now().UTC()

	for _, op := range []*domain.CrossCurrencySwapOperation{
		residueOp("a", domain.ResidueReturnFailed, now.Add(-2*time.Minute), nil),
		residueOp("b", domain.ResidueReturnFailed, now.Add(-time.Minute), nil),
	} {
		if err := db.Create(op).Error; err != nil {
			t.Fatalf("seed %s: %v", op.SwapID, err)
		}
	}

	first, err := r.ClaimRetryableResidues(ctx, now, 50)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if len(first) != 2 {
		t.Fatalf("first claim returned %d rows, want 2", len(first))
	}

	// A second sweeper, same instant: the rows are claimed, so it must find nothing to do.
	second, err := r.ClaimRetryableResidues(ctx, now, 50)
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if len(second) != 0 {
		ids := make([]string, 0, len(second))
		for i := range second {
			ids = append(ids, second[i].SwapID)
		}
		t.Fatalf("a concurrent sweeper picked up already-claimed rows: %v", ids)
	}

	// And the lease is a deferral, not a disappearance: once it expires the row is due again.
	later, err := r.ClaimRetryableResidues(ctx, now.Add(services.ResidueClaimLease()+time.Second), 50)
	if err != nil {
		t.Fatalf("claim after lease: %v", err)
	}
	if len(later) != 2 {
		t.Fatalf("after the lease expired the claim returned %d rows, want 2 — a crashed sweep must not park a residue forever", len(later))
	}
}

// Claiming is not attempting. If the lease consumed an attempt, a busy sweep would walk rows to
// the escalation ceiling without ever having dispatched anything.
func TestClaimRetryableResidues_DoesNotConsumeAnAttempt(t *testing.T) {
	db := repoDB(t, &domain.CrossCurrencySwapOperation{})
	r := newCrossCurrencySwapRepository(db)
	now := time.Now().UTC()

	op := residueOp("a", domain.ResidueReturnFailed, now.Add(-time.Minute), nil)
	op.ResidueAttempts = 2
	if err := db.Create(op).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := r.ClaimRetryableResidues(context.Background(), now, 50); err != nil {
		t.Fatalf("claim: %v", err)
	}

	var got domain.CrossCurrencySwapOperation
	if err := db.Where("swap_id = ?", "a").First(&got).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.ResidueAttempts != 2 {
		t.Errorf("residue_attempts = %d after a claim, want 2 — the claim must not count as an attempt", got.ResidueAttempts)
	}
	if got.ResidueStatus != domain.ResidueReturnFailed {
		t.Errorf("residue_status = %q after a claim, want it unchanged", got.ResidueStatus)
	}
}

// The exclusion SQLite cannot show. FOR UPDATE SKIP LOCKED is what stops two sweepers claiming
// the same rows inside the claim transaction, and it is Postgres-only — SQLite has a single
// writer and rejects the clause, so an ungated version would break every repository test rather
// than fail in production.
//
// This pins the gate. The SQL it produces on Postgres was verified against a real server; see
// the PR. Opening a Postgres dialector here would dial one, which a unit test must not need.
func TestWithResidueClaimLock_LocksOnPostgresOnly(t *testing.T) {
	db := repoDB(t, &domain.CrossCurrencySwapOperation{})

	locked := withResidueClaimLock(db.Session(&gorm.Session{}), "postgres")
	if _, ok := locked.Statement.Clauses["FOR"]; !ok {
		t.Error("postgres claim does not lock the rows it selects")
	}

	for _, dialect := range []string{"sqlite", "mysql", ""} {
		plain := withResidueClaimLock(db.Session(&gorm.Session{}), dialect)
		if _, ok := plain.Statement.Clauses["FOR"]; ok {
			t.Errorf("%q got a locking clause it may not parse", dialect)
		}
	}
}

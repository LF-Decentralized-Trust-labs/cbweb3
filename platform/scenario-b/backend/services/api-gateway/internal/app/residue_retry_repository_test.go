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

func TestListRetryableResidues_SelectsOnlyFailedAndDue(t *testing.T) {
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

	got, err := r.ListRetryableResidues(ctx, now, 50)
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

func TestListRetryableResidues_RespectsLimit(t *testing.T) {
	db := repoDB(t, &domain.CrossCurrencySwapOperation{})
	r := newCrossCurrencySwapRepository(db)
	now := time.Now().UTC()

	for i, id := range []string{"a", "b", "c"} {
		op := residueOp("failed-"+id, domain.ResidueReturnFailed, now.Add(time.Duration(i)*time.Second), nil)
		if err := db.Create(op).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	got, err := r.ListRetryableResidues(context.Background(), now.Add(time.Hour), 2)
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
	all, err := r.ListRetryableResidues(context.Background(), now.Add(time.Hour), 0)
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
func TestListRetryableResidues_ExcludesRowsAtTheAttemptCeiling(t *testing.T) {
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

	got, err := r.ListRetryableResidues(context.Background(), now.Add(time.Hour), 50)
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

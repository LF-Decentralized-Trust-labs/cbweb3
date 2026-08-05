// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
)

func hubSwapClaim(positionID string) *services.HubSwapRecord {
	return &services.HubSwapRecord{
		BridgeInPositionID: positionID,
		CorrelationID:      "corr-1",
		PayerBankID:        "bank-a",
		PoolPair:           "W-BRL-W-COP",
		AmountOut:          "500",
	}
}

func TestHubSwapRepository_ClaimFinalizeThenFind(t *testing.T) {
	db := repoDB(t, &domain.CrossCurrencyHubSwap{})
	r := newCrossCurrencyHubSwapRepository(db)
	ctx := context.Background()

	// Absence is not an error: the caller must be able to tell "no swap yet" from "lookup
	// broke", since only the latter may block a swap.
	missing, err := r.FindByBridgeInPosition(ctx, "pos-1")
	if err != nil {
		t.Fatalf("expected no error for an unknown position, got %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil for an unknown position, got %+v", missing)
	}

	existing, claimed, err := r.Claim(ctx, hubSwapClaim("pos-1"))
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if !claimed || existing != nil {
		t.Fatalf("expected a fresh position to be claimable, got claimed=%v existing=%+v", claimed, existing)
	}

	// A claim is visible while the trade runs, and it is NOT an executed swap: answering a
	// concurrent delivery with this record's (empty) tx hash is what would let it proceed on
	// nothing.
	pending, err := r.FindByBridgeInPosition(ctx, "pos-1")
	if err != nil {
		t.Fatalf("find pending: %v", err)
	}
	if pending == nil || pending.Status != services.HubSwapStatusPending || pending.SwapTxHash != "" {
		t.Fatalf("expected a PENDING claim with no tx hash, got %+v", pending)
	}

	stored, err := r.Finalize(ctx, "pos-1", "800", "0xswap")
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if stored.SwapTxHash != "0xswap" || stored.AmountIn != "800" || stored.Status != services.HubSwapStatusExecuted {
		t.Fatalf("unexpected finalized record: %+v", stored)
	}
}

// A position funds exactly one trade, and the guard has to hold BEFORE the AMM is touched: the
// second delivery must be refused by the constraint, not deduplicated after both have traded.
func TestHubSwapRepository_SecondClaimIsRefusedAndReportsTheFirst(t *testing.T) {
	db := repoDB(t, &domain.CrossCurrencyHubSwap{})
	r := newCrossCurrencyHubSwapRepository(db)
	ctx := context.Background()

	if _, claimed, err := r.Claim(ctx, hubSwapClaim("pos-1")); err != nil || !claimed {
		t.Fatalf("first claim must succeed: claimed=%v err=%v", claimed, err)
	}

	existing, claimed, err := r.Claim(ctx, hubSwapClaim("pos-1"))
	if err != nil {
		t.Fatalf("a losing claim is not an error, it is the guard working: %v", err)
	}
	if claimed {
		t.Fatal("two deliveries must not both own the claim — that is the double-spend window")
	}
	if existing == nil || existing.Status != services.HubSwapStatusPending {
		t.Fatalf("expected the in-flight claim to be reported, got %+v", existing)
	}

	var count int64
	db.Model(&domain.CrossCurrencyHubSwap{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly one claim per bridge-in position, got %d", count)
	}
}

// Once the trade is recorded, a replay reads the realized outcome — which is what lets the
// orchestrator continue on the same facts instead of trading again.
func TestHubSwapRepository_ClaimAfterExecutionReportsTheOutcome(t *testing.T) {
	db := repoDB(t, &domain.CrossCurrencyHubSwap{})
	r := newCrossCurrencyHubSwapRepository(db)
	ctx := context.Background()

	if _, _, err := r.Claim(ctx, hubSwapClaim("pos-1")); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if _, err := r.Finalize(ctx, "pos-1", "800", "0xwinner"); err != nil {
		t.Fatalf("finalize: %v", err)
	}

	existing, claimed, err := r.Claim(ctx, hubSwapClaim("pos-1"))
	if err != nil || claimed {
		t.Fatalf("an executed position must not be re-claimable: claimed=%v err=%v", claimed, err)
	}
	if existing.Status != services.HubSwapStatusExecuted || existing.SwapTxHash != "0xwinner" || existing.AmountIn != "800" {
		t.Fatalf("expected the executed record, got %+v", existing)
	}
}

// An abandoned claim keeps the position guarded. A trade can fail after broadcast, so releasing it
// for a blind retry would reopen the very window the claim closes; the reason travels with the row
// so an operator does not have to correlate logs.
func TestHubSwapRepository_AbandonedClaimStillGuardsThePosition(t *testing.T) {
	db := repoDB(t, &domain.CrossCurrencyHubSwap{})
	r := newCrossCurrencyHubSwapRepository(db)
	ctx := context.Background()

	if _, _, err := r.Claim(ctx, hubSwapClaim("pos-1")); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if err := r.Abandon(ctx, "pos-1", "hub RPC timeout"); err != nil {
		t.Fatalf("abandon: %v", err)
	}

	existing, claimed, err := r.Claim(ctx, hubSwapClaim("pos-1"))
	if err != nil || claimed {
		t.Fatalf("a failed claim must not be re-claimable without reconciliation: claimed=%v err=%v", claimed, err)
	}
	if existing.Status != services.HubSwapStatusFailed || existing.FailureReason != "hub RPC timeout" {
		t.Fatalf("expected a FAILED claim carrying its reason, got %+v", existing)
	}
}

// Finalize is scoped to a PENDING claim: if anything else already moved the row, this call must not
// overwrite it and the caller has to learn that instead of believing it recorded the trade.
func TestHubSwapRepository_FinalizeRefusesWhenThereIsNoPendingClaim(t *testing.T) {
	db := repoDB(t, &domain.CrossCurrencyHubSwap{})
	r := newCrossCurrencyHubSwapRepository(db)
	ctx := context.Background()

	if _, err := r.Finalize(ctx, "pos-unknown", "800", "0xswap"); err == nil {
		t.Fatal("finalizing a position with no claim must be an error, not a silent no-op")
	}

	if _, _, err := r.Claim(ctx, hubSwapClaim("pos-1")); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if _, err := r.Finalize(ctx, "pos-1", "800", "0xfirst"); err != nil {
		t.Fatalf("finalize: %v", err)
	}
	if _, err := r.Finalize(ctx, "pos-1", "900", "0xsecond"); err == nil {
		t.Fatal("a second finalize must not overwrite the recorded outcome")
	}
}

// Rows written before the claim existed carry no status and a transaction hash. Reading them as
// PENDING would make every past swap look like a delegation in flight, so an in-place upgrade would
// answer 409 to legitimate replays.
func TestHubSwapRepository_LegacyRowWithoutStatusReadsAsExecuted(t *testing.T) {
	db := repoDB(t, &domain.CrossCurrencyHubSwap{})
	r := newCrossCurrencyHubSwapRepository(db)
	ctx := context.Background()

	if err := db.Create(&domain.CrossCurrencyHubSwap{
		BridgeInPositionID: "pos-legacy",
		CorrelationID:      "corr-old",
		PayerBankID:        "bank-a",
		PoolPair:           "W-BRL-W-COP",
		AmountOut:          "500",
		AmountIn:           "800",
		SwapTxHash:         "0xold",
	}).Error; err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}

	found, err := r.FindByBridgeInPosition(ctx, "pos-legacy")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found.Status != services.HubSwapStatusExecuted {
		t.Fatalf("a legacy row with a tx hash must read as EXECUTED, got %q", found.Status)
	}
}

func TestHubSwapRepository_DistinctPositionsCoexist(t *testing.T) {
	db := repoDB(t, &domain.CrossCurrencyHubSwap{})
	r := newCrossCurrencyHubSwapRepository(db)
	ctx := context.Background()

	for _, id := range []string{"pos-1", "pos-2"} {
		if _, _, err := r.Claim(ctx, hubSwapClaim(id)); err != nil {
			t.Fatalf("claim %s: %v", id, err)
		}
	}
	if _, err := r.Finalize(ctx, "pos-2", "900", "0xb"); err != nil {
		t.Fatalf("finalize pos-2: %v", err)
	}

	found, err := r.FindByBridgeInPosition(ctx, "pos-2")
	if err != nil || found == nil || found.SwapTxHash != "0xb" {
		t.Fatalf("expected pos-2's own swap, got %+v (err=%v)", found, err)
	}
}

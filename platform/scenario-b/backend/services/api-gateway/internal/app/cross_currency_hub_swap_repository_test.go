// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
)

func hubSwapRec(positionID, txHash, amountIn string) *services.HubSwapRecord {
	return &services.HubSwapRecord{
		BridgeInPositionID: positionID,
		CorrelationID:      "corr-1",
		PayerBankID:        "bank-a",
		PoolPair:           "W-BRL-W-COP",
		AmountOut:          "500",
		AmountIn:           amountIn,
		SwapTxHash:         txHash,
	}
}

func TestHubSwapRepository_RecordThenFind(t *testing.T) {
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

	stored, err := r.Record(ctx, hubSwapRec("pos-1", "0xswap", "800"))
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if stored.SwapTxHash != "0xswap" || stored.AmountIn != "800" {
		t.Fatalf("unexpected stored record: %+v", stored)
	}

	found, err := r.FindByBridgeInPosition(ctx, "pos-1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found == nil || found.SwapTxHash != "0xswap" {
		t.Fatalf("expected the recorded swap to be found, got %+v", found)
	}
}

// TestHubSwapRepository_SecondRecordReturnsTheFirst is the replay guard at the storage layer:
// a bridge-in position funds one swap, so a losing concurrent insert must resolve to the
// stored record instead of surfacing a constraint error the caller would treat as a failure.
func TestHubSwapRepository_SecondRecordReturnsTheFirst(t *testing.T) {
	db := repoDB(t, &domain.CrossCurrencyHubSwap{})
	r := newCrossCurrencyHubSwapRepository(db)
	ctx := context.Background()

	if _, err := r.Record(ctx, hubSwapRec("pos-1", "0xwinner", "800")); err != nil {
		t.Fatalf("first record: %v", err)
	}

	second, err := r.Record(ctx, hubSwapRec("pos-1", "0xloser", "810"))
	if err != nil {
		t.Fatalf("expected the losing insert to resolve to the stored record, got %v", err)
	}
	if second.SwapTxHash != "0xwinner" || second.AmountIn != "800" {
		t.Fatalf("expected the winning record, got %+v", second)
	}

	var count int64
	db.Model(&domain.CrossCurrencyHubSwap{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly one swap per bridge-in position, got %d", count)
	}
}

func TestHubSwapRepository_DistinctPositionsCoexist(t *testing.T) {
	db := repoDB(t, &domain.CrossCurrencyHubSwap{})
	r := newCrossCurrencyHubSwapRepository(db)
	ctx := context.Background()

	if _, err := r.Record(ctx, hubSwapRec("pos-1", "0xa", "800")); err != nil {
		t.Fatalf("record pos-1: %v", err)
	}
	if _, err := r.Record(ctx, hubSwapRec("pos-2", "0xb", "900")); err != nil {
		t.Fatalf("record pos-2: %v", err)
	}

	found, err := r.FindByBridgeInPosition(ctx, "pos-2")
	if err != nil || found == nil || found.SwapTxHash != "0xb" {
		t.Fatalf("expected pos-2's own swap, got %+v (err=%v)", found, err)
	}
}

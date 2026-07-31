// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// The load-bearing schema property: a single Hub swap legitimately produces two positions —
// the settlement and the residue return. Before the composite (swap_tx_hash, leg) index, the
// residue leg collided with the settlement on a unique constraint and was silently dropped as
// a replay. In a single-CB deployment both legs live in the same table, so this is not
// hypothetical.
func TestResidueLeg_CoexistsWithSettlementOnSameSwapTxHash(t *testing.T) {
	db := newBridgeDB(t)
	svc := NewBridgeBurnUnlockService(db)
	ctx := context.Background()
	const swapTx = "0xfeedface"

	settlement, err := svc.EnqueueBurnAfterSwap(ctx,
		"bank-b", "spoke-ars", "tCeBM-ARS", "W-ARS", "100", "corr-1",
		"0xHUBSIGNER", "0xBENEFICIARY", swapTx)
	if err != nil {
		t.Fatalf("settlement leg: %v", err)
	}

	residue, err := svc.EnqueueResidueReturn(ctx,
		"bank-a", "spoke-brl", "tCeBM-BRL", "W-BRL", "200", "corr-1",
		"0xHUBSIGNER", "0xPAYER", swapTx, "pos-bridge-in-1")
	if err != nil {
		t.Fatalf("residue leg must be accepted alongside the settlement, got: %v", err)
	}
	if residue.PositionID == settlement.PositionID {
		t.Fatal("residue leg must be a distinct position, not the settlement returned as a duplicate")
	}
}

// Replay protection still holds *within* a leg.
func TestResidueLeg_SecondReturnForSameSwapIsIdempotent(t *testing.T) {
	db := newBridgeDB(t)
	svc := NewBridgeBurnUnlockService(db)
	ctx := context.Background()
	const swapTx = "0xfeedface"

	first, err := svc.EnqueueResidueReturn(ctx,
		"bank-a", "spoke-brl", "tCeBM-BRL", "W-BRL", "200", "corr-1",
		"0xHUBSIGNER", "0xPAYER", swapTx, "pos-bridge-in-1")
	if err != nil {
		t.Fatalf("first residue leg: %v", err)
	}

	second, err := svc.EnqueueResidueReturn(ctx,
		"bank-a", "spoke-brl", "tCeBM-BRL", "W-BRL", "200", "corr-1",
		"0xHUBSIGNER", "0xPAYER", swapTx, "pos-bridge-in-1")
	if err != nil {
		t.Fatalf("replay should be idempotent, got: %v", err)
	}
	if second.PositionID != first.PositionID {
		t.Fatalf("replay must return the existing position %s, got %s", first.PositionID, second.PositionID)
	}

	var count int64
	db.Model(&domain.BridgedAssetPosition{}).Where("leg = ?", domain.BridgeLegResidue).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly one residue position, got %d", count)
	}
}

// The settlement-leg lookup must not be satisfied by a residue leg carrying the same swap
// hash, or a legitimate payment would be rejected as an already-consumed swap.
func TestFindBySwapTxHash_IgnoresResidueLeg(t *testing.T) {
	db := newBridgeDB(t)
	svc := NewBridgeBurnUnlockService(db)
	ctx := context.Background()
	const swapTx = "0xfeedface"

	if _, err := svc.EnqueueResidueReturn(ctx,
		"bank-a", "spoke-brl", "tCeBM-BRL", "W-BRL", "200", "corr-1",
		"0xHUBSIGNER", "0xPAYER", swapTx, "pos-bridge-in-1"); err != nil {
		t.Fatalf("residue leg: %v", err)
	}

	found, err := svc.FindBySwapTxHash(ctx, swapTx)
	if err != nil {
		t.Fatalf("settlement lookup: %v", err)
	}
	if found != nil {
		t.Fatalf("a residue leg must not register as a consumed settlement, got position %s", found.PositionID)
	}

	residueFound, err := svc.FindResidueBySwapTxHash(ctx, swapTx)
	if err != nil {
		t.Fatalf("residue lookup: %v", err)
	}
	if residueFound == nil {
		t.Fatal("the residue-leg lookup must find it")
	}
}

// Legacy rows predate the leg column. They are settlements, and the settlement lookup must
// keep finding them so replay protection is not lost on a partially migrated table.
func TestFindBySwapTxHash_StillFindsLegacyRowWithEmptyLeg(t *testing.T) {
	db := newBridgeDB(t)
	svc := NewBridgeBurnUnlockService(db)
	ctx := context.Background()

	legacy := &domain.BridgedAssetPosition{
		PositionID:     "legacy-1",
		OwnerBankID:    "bank-b",
		SpokeNetwork:   "spoke-ars",
		NativeAsset:    "tCeBM-ARS",
		MirroredAsset:  "W-ARS",
		MirroredAmount: "100",
		BridgeState:    domain.BridgeStateActive,
		SwapTxHash:     "0xlegacy",
		Leg:            "", // pre-migration row
	}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}

	found, err := svc.FindBySwapTxHash(ctx, "0xlegacy")
	if err != nil {
		t.Fatalf("settlement lookup: %v", err)
	}
	if found == nil || found.PositionID != "legacy-1" {
		t.Fatal("legacy settlement rows must remain visible to replay protection")
	}
}

// Net consumption is derived from the pair of positions; the bridge-in record is never
// rewritten, since it is on-chain evidence of what was moved.
func TestNetConsumed_SubtractsResidueLegsFromBridgedAmount(t *testing.T) {
	db := newBridgeDB(t)
	ctx := context.Background()

	parent := &domain.BridgedAssetPosition{
		PositionID:     "pos-bridge-in-1",
		OwnerBankID:    "bank-a",
		SpokeNetwork:   "spoke-brl",
		NativeAsset:    "tCeBM-BRL",
		MirroredAsset:  "W-BRL",
		MirroredAmount: "1000",
		BridgeState:    domain.BridgeStateActive,
		Leg:            domain.BridgeLegSettlement,
	}
	if err := db.Create(parent).Error; err != nil {
		t.Fatalf("seed bridge-in: %v", err)
	}

	reader := NewBridgePositionReader(db)

	net, err := reader.NetConsumed(ctx, "pos-bridge-in-1")
	if err != nil {
		t.Fatalf("net consumed without residue: %v", err)
	}
	if net != "1000" {
		t.Fatalf("with no residue leg the net is the bridged amount, got %q", net)
	}

	svc := NewBridgeBurnUnlockService(db)
	if _, err := svc.EnqueueResidueReturn(ctx,
		"bank-a", "spoke-brl", "tCeBM-BRL", "W-BRL", "200", "corr-1",
		"0xHUBSIGNER", "0xPAYER", "0xfeed", "pos-bridge-in-1"); err != nil {
		t.Fatalf("residue leg: %v", err)
	}

	net, err = reader.NetConsumed(ctx, "pos-bridge-in-1")
	if err != nil {
		t.Fatalf("net consumed with residue: %v", err)
	}
	if net != "800" {
		t.Fatalf("expected 1000 − 200 = 800, got %q", net)
	}

	// The parent must be untouched.
	var reloaded domain.BridgedAssetPosition
	if err := db.Where("position_id = ?", "pos-bridge-in-1").First(&reloaded).Error; err != nil {
		t.Fatalf("reload parent: %v", err)
	}
	if reloaded.MirroredAmount != "1000" {
		t.Fatalf("the bridge-in record must not be rewritten, got %q", reloaded.MirroredAmount)
	}
}

// A residue leg that never reaches RELEASED is exactly the over-debit the whole feature
// exists to remove, so it must be queryable.
func TestListStrandedResidueLegs(t *testing.T) {
	db := newBridgeDB(t)
	svc := NewBridgeBurnUnlockService(db)
	reader := NewBridgePositionReader(db)
	ctx := context.Background()

	stranded, err := svc.EnqueueResidueReturn(ctx,
		"bank-a", "spoke-brl", "tCeBM-BRL", "W-BRL", "200", "corr-1",
		"0xHUBSIGNER", "0xPAYER", "0xfeed1", "pos-1")
	if err != nil {
		t.Fatalf("residue leg: %v", err)
	}
	returned, err := svc.EnqueueResidueReturn(ctx,
		"bank-a", "spoke-brl", "tCeBM-BRL", "W-BRL", "300", "corr-2",
		"0xHUBSIGNER", "0xPAYER", "0xfeed2", "pos-2")
	if err != nil {
		t.Fatalf("residue leg: %v", err)
	}
	// Drive the second one to completion.
	if err := db.Model(&domain.BridgedAssetPosition{}).
		Where("position_id = ?", returned.PositionID).
		Update("bridge_state", domain.BridgeStateReleased).Error; err != nil {
		t.Fatalf("release second leg: %v", err)
	}

	legs, err := reader.ListStrandedResidueLegs(ctx)
	if err != nil {
		t.Fatalf("list stranded: %v", err)
	}
	if len(legs) != 1 {
		t.Fatalf("expected only the unreleased leg, got %d", len(legs))
	}
	if legs[0].PositionID != stranded.PositionID {
		t.Fatalf("expected %s, got %s", stranded.PositionID, legs[0].PositionID)
	}
}

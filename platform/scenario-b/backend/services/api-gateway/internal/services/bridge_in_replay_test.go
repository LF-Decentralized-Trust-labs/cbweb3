// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// A replayed bridge-in must not create a second position. Each position drives one
// spoke-side burn of the payer bank's tCeBM and one Hub mint, so a duplicate is a
// double-charge of the bank's reserves and an inflation of wrapped supply.
func TestBridgeIn_ReplayedCorrelationReturnsExistingPosition(t *testing.T) {
	db := newBridgeDB(t)
	svc := NewBridgeLockMintService(db)
	ctx := context.Background()

	first, err := svc.LockAndEnqueue(ctx, "bank-a", "spoke-brl", "tCeBM-BRL", "W-BRL", "1000", "corr-replay",
		"0xSWAPSENDER", "0xPAYER")
	if err != nil {
		t.Fatalf("first bridge-in: %v", err)
	}

	second, err := svc.LockAndEnqueue(ctx, "bank-a", "spoke-brl", "tCeBM-BRL", "W-BRL", "1000", "corr-replay",
		"0xSWAPSENDER", "0xPAYER")
	if err != nil {
		t.Fatalf("replay must be idempotent, got: %v", err)
	}
	if second.PositionID != first.PositionID {
		t.Fatalf("replay created a second position: %s then %s", first.PositionID, second.PositionID)
	}

	var positions int64
	db.Model(&domain.BridgedAssetPosition{}).Where("correlation_id = ?", "corr-replay").Count(&positions)
	if positions != 1 {
		t.Fatalf("expected exactly 1 position for the correlation, found %d", positions)
	}

	var queued int64
	db.Model(&domain.RelayerQueueItem{}).Where("position_id = ?", first.PositionID).Count(&queued)
	if queued != 1 {
		t.Fatalf("expected exactly 1 lock-mint queue item, found %d — the relayer would burn twice", queued)
	}
}

// The scope of the replay key is the point. A swap legitimately produces outbound legs that
// carry the same correlation id — the settlement bridge-out and the residue return — and
// blocking those would break the flow the guard is meant to protect.
func TestBridgeIn_OutboundLegsShareTheCorrelationWithoutBeingReplays(t *testing.T) {
	db := newBridgeDB(t)
	lockMint := NewBridgeLockMintService(db)
	burnUnlock := NewBridgeBurnUnlockService(db)
	ctx := context.Background()
	const corr = "corr-full-swap"

	bridgeIn, err := lockMint.LockAndEnqueue(ctx, "bank-a", "spoke-brl", "tCeBM-BRL", "W-BRL", "1000", corr,
		"0xSWAPSENDER", "0xPAYER")
	if err != nil {
		t.Fatalf("bridge-in: %v", err)
	}

	settlement, err := burnUnlock.EnqueueBurnAfterSwap(ctx,
		"bank-b", "spoke-ars", "tCeBM-ARS", "W-ARS", "900", corr,
		"0xHUBSIGNER", "0xBENEFICIARY", "0xswaptx")
	if err != nil {
		t.Fatalf("settlement bridge-out must not be treated as a bridge-in replay: %v", err)
	}

	residue, err := burnUnlock.EnqueueResidueReturn(ctx,
		"bank-a", "spoke-brl", "tCeBM-BRL", "W-BRL", "100", corr,
		"0xHUBSIGNER", "0xPAYER", "0xswaptx", bridgeIn.PositionID)
	if err != nil {
		t.Fatalf("residue return must not be treated as a bridge-in replay: %v", err)
	}

	if settlement.PositionID == bridgeIn.PositionID || residue.PositionID == bridgeIn.PositionID {
		t.Fatal("outbound legs were collapsed onto the bridge-in position")
	}
}

// A plain lock-mint carries no correlation id — there is no notification behind it to replay,
// and two of them are two distinct intents. Deduplicating on the empty string would collapse
// every such position into the first one ever created.
func TestBridgeIn_EmptyCorrelationIsNotDeduplicated(t *testing.T) {
	db := newBridgeDB(t)
	svc := NewBridgeLockMintService(db)
	ctx := context.Background()

	first, err := svc.LockAndEnqueue(ctx, "bank-a", "spoke-brl", "tCeBM-BRL", "W-BRL", "10", "")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := svc.LockAndEnqueue(ctx, "bank-a", "spoke-brl", "tCeBM-BRL", "W-BRL", "20", "")
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first.PositionID == second.PositionID {
		t.Fatal("two independent lock-mints were collapsed into one position")
	}
}

// The database enforces the rule too, not just the pre-check. Without this the guard would be
// a read-then-write with a window in the middle, which is exactly the window a retried
// notification lands in.
func TestBridgeIn_UniqueIndexRejectsASecondInboundRow(t *testing.T) {
	db := newBridgeDB(t)
	ctx := context.Background()
	const corr = "corr-index"

	for i, want := range []bool{true, false} {
		err := db.WithContext(ctx).Create(&domain.BridgedAssetPosition{
			PositionID:    "pos-" + string(rune('a'+i)),
			OwnerBankID:   "bank-a",
			SpokeNetwork:  "spoke-brl",
			NativeAsset:   "tCeBM-BRL",
			MirroredAsset: "W-BRL",
			Direction:     domain.BridgeDirectionIn,
			CorrelationID: corr,
			BridgeState:   domain.BridgeStateLocking,
		}).Error
		if got := err == nil; got != want {
			t.Fatalf("insert %d: accepted=%v, want %v (err=%v)", i, got, want, err)
		}
	}
}

// An interrupted first attempt leaves a position with no queue item — nothing drives it, and
// the caller saw an error. The replay must repair that rather than report success for a
// position the relayer will never pick up.
func TestBridgeIn_ReplayRepairsAPositionLeftWithoutAQueueItem(t *testing.T) {
	db := newBridgeDB(t)
	svc := NewBridgeLockMintService(db)
	ctx := context.Background()
	const corr = "corr-orphan"

	orphan := &domain.BridgedAssetPosition{
		PositionID:    "pos-orphan",
		OwnerBankID:   "bank-a",
		SpokeNetwork:  "spoke-brl",
		NativeAsset:   "tCeBM-BRL",
		MirroredAsset: "W-BRL",
		Direction:     domain.BridgeDirectionIn,
		CorrelationID: corr,
		BridgeState:   domain.BridgeStateLocking,
	}
	if err := db.WithContext(ctx).Create(orphan).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := svc.LockAndEnqueue(ctx, "bank-a", "spoke-brl", "tCeBM-BRL", "W-BRL", "1000", corr)
	if err != nil {
		t.Fatalf("replay over an orphaned position: %v", err)
	}
	if got.PositionID != orphan.PositionID {
		t.Fatalf("returned %s, want the existing %s", got.PositionID, orphan.PositionID)
	}

	var queued int64
	db.Model(&domain.RelayerQueueItem{}).Where("position_id = ?", orphan.PositionID).Count(&queued)
	if queued != 1 {
		t.Fatalf("expected the replay to create the missing queue item, found %d", queued)
	}
}

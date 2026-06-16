// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

func TestBridgeLockMint_LockAndEnqueue(t *testing.T) {
	db := newBridgeDB(t)
	svc := NewBridgeLockMintService(db)

	res, err := svc.LockAndEnqueue(context.Background(), "bank-a", "spoke-a", "BRL", "W-BRL", "1000", "corr-1", "0xmintto", "0xburnfrom")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.BridgeState != string(domain.BridgeStateLocking) {
		t.Fatalf("expected LOCKING state, got %s", res.BridgeState)
	}

	var posCount, itemCount int64
	db.Model(&domain.BridgedAssetPosition{}).Count(&posCount)
	db.Model(&domain.RelayerQueueItem{}).Count(&itemCount)
	if posCount != 1 || itemCount != 1 {
		t.Fatalf("expected 1 position + 1 queue item, got %d / %d", posCount, itemCount)
	}
}

func TestBridgeLockMint_Validation(t *testing.T) {
	svc := NewBridgeLockMintService(newBridgeDB(t))
	if _, err := svc.LockAndEnqueue(context.Background(), "", "spoke-a", "BRL", "W-BRL", "1000", ""); err == nil {
		t.Fatal("expected validation error for missing owner")
	}
}

func TestBridgeBurnUnlock_BurnAndEnqueue(t *testing.T) {
	db := newBridgeDB(t)
	// Seed an ACTIVE position.
	pos := &domain.BridgedAssetPosition{
		PositionID:    "pos-1",
		OwnerBankID:   "bank-a",
		SpokeNetwork:  "spoke-b",
		NativeAsset:   "ARS",
		MirroredAsset: "W-ARS",
		MirroredAmount: "500",
		BridgeState:   domain.BridgeStateActive,
	}
	if err := db.Create(pos).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	svc := NewBridgeBurnUnlockService(db)
	res, err := svc.BurnAndEnqueue(context.Background(), "pos-1", "corr-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.BridgeState != string(domain.BridgeStateBurning) {
		t.Fatalf("expected BURNING, got %s", res.BridgeState)
	}
}

func TestBridgeBurnUnlock_Validation(t *testing.T) {
	svc := NewBridgeBurnUnlockService(newBridgeDB(t))
	if _, err := svc.BurnAndEnqueue(context.Background(), "", ""); err == nil {
		t.Fatal("expected validation error for empty position id")
	}
}

func TestBridgeBurnUnlock_NotFound(t *testing.T) {
	svc := NewBridgeBurnUnlockService(newBridgeDB(t))
	if _, err := svc.BurnAndEnqueue(context.Background(), "missing", ""); err == nil {
		t.Fatal("expected not-found error")
	}
}

func TestBridgeBurnUnlock_EnqueueBurnAfterSwap(t *testing.T) {
	db := newBridgeDB(t)
	svc := NewBridgeBurnUnlockService(db)
	res, err := svc.EnqueueBurnAfterSwap(context.Background(), "bank-b", "spoke-b", "ARS", "W-ARS", "500", "corr-3", "0xburn", "0xbene", "0xswaptx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.BridgeState != string(domain.BridgeStateActive) {
		t.Fatalf("expected ACTIVE bridge-out position, got %s", res.BridgeState)
	}

	// FindBySwapTxHash should locate it.
	found, err := svc.FindBySwapTxHash(context.Background(), "0xswaptx")
	if err != nil || found == nil {
		t.Fatalf("expected to find position by swap tx hash, got %v err %v", found, err)
	}

	// Empty hash → nil, nil.
	none, err := svc.FindBySwapTxHash(context.Background(), "")
	if err != nil || none != nil {
		t.Fatalf("expected (nil,nil) for empty hash, got %v err %v", none, err)
	}

	// Unknown hash → nil, nil (record not found).
	none, err = svc.FindBySwapTxHash(context.Background(), "0xunknown")
	if err != nil || none != nil {
		t.Fatalf("expected (nil,nil) for unknown hash, got %v err %v", none, err)
	}
}

func TestBridgeBurnUnlock_EnqueueBurnAfterSwap_Validation(t *testing.T) {
	svc := NewBridgeBurnUnlockService(newBridgeDB(t))
	if _, err := svc.EnqueueBurnAfterSwap(context.Background(), "", "spoke", "n", "w", "1", ""); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestBridgePositionReader(t *testing.T) {
	db := newBridgeDB(t)
	reader := NewBridgePositionReader(db)

	for _, st := range []domain.BridgeState{domain.BridgeStateActive, domain.BridgeStateLocking} {
		db.Create(&domain.BridgedAssetPosition{
			PositionID:  "pos-" + string(st),
			OwnerBankID: "bank-a",
			BridgeState: st,
		})
	}

	all, err := reader.ListPositions(context.Background(), "")
	if err != nil || len(all) != 2 {
		t.Fatalf("expected 2 positions, got %d err %v", len(all), err)
	}

	active, err := reader.ListPositions(context.Background(), string(domain.BridgeStateActive))
	if err != nil || len(active) != 1 {
		t.Fatalf("expected 1 active, got %d err %v", len(active), err)
	}

	state, err := reader.GetBridgeState(context.Background(), "pos-ACTIVE")
	if err != nil || state != domain.BridgeStateActive {
		t.Fatalf("expected ACTIVE, got %s err %v", state, err)
	}

	if _, err := reader.GetBridgeState(context.Background(), "nope"); err == nil {
		t.Fatal("expected error for unknown position")
	}

	has, err := reader.HasActiveBridgePosition(context.Background(), "bank-a")
	if err != nil || !has {
		t.Fatalf("expected active position for bank-a, got %v err %v", has, err)
	}
	has, err = reader.HasActiveBridgePosition(context.Background(), "bank-z")
	if err != nil || has {
		t.Fatalf("expected no active position for bank-z, got %v err %v", has, err)
	}
}

func TestSanitizeLogFieldAndScaleAmount(t *testing.T) {
	if got := sanitizeLogField("a\r\nb"); got != "ab" {
		t.Fatalf("sanitize failed: %q", got)
	}
	if got := scaleAmountBps("1000", 5000); got != "500" {
		t.Fatalf("scaleAmountBps(1000, 5000) = %s, want 500", got)
	}
	if got := scaleAmountBps("notnum", 5000); got != "notnum" {
		t.Fatalf("scaleAmountBps non-numeric should pass through, got %s", got)
	}
}

// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	podmain "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"gorm.io/gorm"
)

func seedPosition(t *testing.T, db *gorm.DB, state podmain.BridgeState) *podmain.BridgedAssetPosition {
	t.Helper()
	pos := &podmain.BridgedAssetPosition{
		PositionID:     "pos-1",
		OwnerBankID:    "bank-a",
		SpokeNetwork:   "spoke-b",
		NativeAsset:    "0xnative",
		MirroredAsset:  "0xmirror",
		MirroredAmount: "2500",
		BridgeState:    state,
	}
	if err := db.Create(pos).Error; err != nil {
		t.Fatalf("seed position: %v", err)
	}
	return pos
}

func TestBurnAndEnqueue_Success(t *testing.T) {
	db := newTestDB(t)
	seedPosition(t, db, podmain.BridgeStateActive)
	bridge := &fakeUnlockCaller{txHash: "0xunlock"}
	relayer := &fakeBurnRelayer{}
	svc := NewBridgeBurnUnlockService(db, bridge, relayer)

	pos, err := svc.BurnAndEnqueue(context.Background(), "pos-1")
	if err != nil {
		t.Fatalf("BurnAndEnqueue: %v", err)
	}
	// ACTIVE → BURNING transition recorded in returned object and DB.
	if pos.BridgeState != podmain.BridgeStateBurning {
		t.Errorf("expected BURNING, got %s", pos.BridgeState)
	}
	var stored podmain.BridgedAssetPosition
	if err := db.Where("position_id = ?", "pos-1").First(&stored).Error; err != nil {
		t.Fatalf("load: %v", err)
	}
	if stored.BridgeState != podmain.BridgeStateBurning {
		t.Errorf("DB state expected BURNING, got %s", stored.BridgeState)
	}

	// Burn-unlock queue item enqueued, PENDING, keyed on position + unlock txHash.
	var item podmain.RelayerQueueItem
	if err := db.First(&item).Error; err != nil {
		t.Fatalf("load queue item: %v", err)
	}
	if item.EventType != podmain.RelayerEventTypeBurnUnlock {
		t.Errorf("expected BURN_UNLOCK, got %s", item.EventType)
	}
	wantKey := "burn:pos-1:0xunlock"
	if item.IdempotencyKey != wantKey {
		t.Errorf("expected key %q, got %q", wantKey, item.IdempotencyKey)
	}
	if relayer.calls != 1 {
		t.Errorf("expected 1 relayer call, got %d", relayer.calls)
	}
}

func TestBurnAndEnqueue_EmptyPositionID(t *testing.T) {
	db := newTestDB(t)
	svc := NewBridgeBurnUnlockService(db, &fakeUnlockCaller{}, &fakeBurnRelayer{})
	_, err := svc.BurnAndEnqueue(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty position_id")
	}
}

// Only ACTIVE positions may be burned. A non-ACTIVE (or missing) position must be
// rejected with no state change and no queue item — prevents double-spend / partial flows.
func TestBurnAndEnqueue_NonActiveRejected(t *testing.T) {
	for _, state := range []podmain.BridgeState{
		podmain.BridgeStateLocking,
		podmain.BridgeStateBurning,
		podmain.BridgeStateBurned,
		podmain.BridgeStateReleased,
	} {
		t.Run(string(state), func(t *testing.T) {
			db := newTestDB(t)
			seedPosition(t, db, state)
			bridge := &fakeUnlockCaller{txHash: "0x1"}
			svc := NewBridgeBurnUnlockService(db, bridge, &fakeBurnRelayer{})

			_, err := svc.BurnAndEnqueue(context.Background(), "pos-1")
			if err == nil {
				t.Fatal("expected error for non-active position")
			}
			if bridge.calls != 0 {
				t.Errorf("unlock must not be called for non-active position")
			}
			if n := countQueueItems(t, db); n != 0 {
				t.Errorf("no queue item expected, got %d", n)
			}
		})
	}
}

func TestBurnAndEnqueue_PositionNotFound(t *testing.T) {
	db := newTestDB(t)
	svc := NewBridgeBurnUnlockService(db, &fakeUnlockCaller{}, &fakeBurnRelayer{})
	_, err := svc.BurnAndEnqueue(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if !strings.Contains(err.Error(), "active bridged position not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

// When the on-chain unlock (leg 1 of burn→unlock) fails, the position must stay
// ACTIVE and no burn queue item may be enqueued. No partial settlement.
func TestBurnAndEnqueue_UnlockFailsStateUnchanged(t *testing.T) {
	db := newTestDB(t)
	seedPosition(t, db, podmain.BridgeStateActive)
	bridge := &fakeUnlockCaller{err: errors.New("revert")}
	svc := NewBridgeBurnUnlockService(db, bridge, &fakeBurnRelayer{})

	_, err := svc.BurnAndEnqueue(context.Background(), "pos-1")
	if err == nil {
		t.Fatal("expected unlock error")
	}
	var stored podmain.BridgedAssetPosition
	if err := db.Where("position_id = ?", "pos-1").First(&stored).Error; err != nil {
		t.Fatalf("load: %v", err)
	}
	if stored.BridgeState != podmain.BridgeStateActive {
		t.Errorf("state must remain ACTIVE after unlock failure, got %s", stored.BridgeState)
	}
	if n := countQueueItems(t, db); n != 0 {
		t.Errorf("no queue item expected after unlock failure, got %d", n)
	}
}

func TestBurnAndEnqueue_RelayerSubmitFailureIsNonFatal(t *testing.T) {
	db := newTestDB(t)
	seedPosition(t, db, podmain.BridgeStateActive)
	bridge := &fakeUnlockCaller{txHash: "0xunlock"}
	relayer := &fakeBurnRelayer{err: errors.New("relayer down")}
	svc := NewBridgeBurnUnlockService(db, bridge, relayer)

	pos, err := svc.BurnAndEnqueue(context.Background(), "pos-1")
	if err != nil {
		t.Fatalf("expected success despite relayer failure: %v", err)
	}
	if pos.BridgeState != podmain.BridgeStateBurning {
		t.Errorf("expected BURNING, got %s", pos.BridgeState)
	}
	if n := countQueueItems(t, db); n != 1 {
		t.Errorf("queue item must persist for worker retry, got %d", n)
	}
}

// SPDX-License-Identifier: Apache-2.0

//go:build integration_lite

// Suite 2 — Cross-ledger consistency / bridge atomicity (D6 integration priority).
//
// Drives the Scenario B bridge lifecycle across a fake spoke and hub using the
// orchestrator's REAL services.BridgeLockMintService and
// services.BridgeBurnUnlockService against in-memory SQLite:
//
//	lock → mint (LOCKING) ; relayer confirms (ACTIVE) ; burn → unlock (BURNING)
//
// Asserts the constitution's NO PARTIAL SETTLEMENT rule: either both legs of a
// transition complete (state advances + relayer item enqueued) or the position
// state is left unchanged with no queue item (clean refund/retry path). Also
// exercises the timeout/refund branch and the non-active double-spend guard.
package integrationlite

import (
	"context"
	"errors"
	"testing"
	"time"

	podomain "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	posvc "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/services"
	"gorm.io/gorm"
)

// --- fake spoke/hub bridge collaborators (implement the service interfaces) ---

type fakeLockCaller struct {
	txHash string
	err    error
	calls  int
}

func (f *fakeLockCaller) LockAsset(_ context.Context, _, _, _, _ string) (*podomain.LockResult, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &podomain.LockResult{TxHash: f.txHash}, nil
}

type fakeUnlockCaller struct {
	txHash string
	err    error
	calls  int
}

func (f *fakeUnlockCaller) UnlockAsset(_ context.Context, _, _, _, _ string) (*podomain.UnlockResult, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &podomain.UnlockResult{TxHash: f.txHash}, nil
}

type fakeRelayer struct {
	lockKeys []string
	burnKeys []string
	err      error
}

func (f *fakeRelayer) SubmitLockEvent(_ context.Context, key, _ string) error {
	f.lockKeys = append(f.lockKeys, key)
	return f.err
}
func (f *fakeRelayer) SubmitBurnEvent(_ context.Context, key, _ string) error {
	f.burnKeys = append(f.burnKeys, key)
	return f.err
}

func newBridgeDB(t *testing.T) *gorm.DB {
	return newMemDB(t, &podomain.BridgedAssetPosition{}, &podomain.RelayerQueueItem{})
}

func countPositions(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	db.Model(&podomain.BridgedAssetPosition{}).Count(&n)
	return n
}

func countQueue(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	db.Model(&podomain.RelayerQueueItem{}).Count(&n)
	return n
}

func loadPosition(t *testing.T, db *gorm.DB, id string) podomain.BridgedAssetPosition {
	t.Helper()
	var pos podomain.BridgedAssetPosition
	if err := db.Where("position_id = ?", id).First(&pos).Error; err != nil {
		t.Fatalf("load position %s: %v", id, err)
	}
	return pos
}

func seedActivePosition(t *testing.T, db *gorm.DB, id, amount string) {
	t.Helper()
	pos := &podomain.BridgedAssetPosition{
		PositionID:     id,
		OwnerBankID:    "bank-a",
		SpokeNetwork:   "spoke-b",
		NativeAsset:    "0xnative",
		MirroredAsset:  "0xmirror",
		MirroredAmount: amount,
		BridgeState:    podomain.BridgeStateActive,
	}
	if err := db.Create(pos).Error; err != nil {
		t.Fatalf("seed active position: %v", err)
	}
}

// TestBridge_LockMintThenBurnUnlock_FullCycle drives both legs end to end across
// the fake spoke and hub. Both legs of each transition complete — no partial
// settlement.
func TestBridge_LockMintThenBurnUnlock_FullCycle(t *testing.T) {
	db := newBridgeDB(t)
	relayer := &fakeRelayer{}
	lockSvc := posvc.NewBridgeLockMintService(db, &fakeLockCaller{txHash: "0xlock"}, relayer)
	burnSvc := posvc.NewBridgeBurnUnlockService(db, &fakeUnlockCaller{txHash: "0xunlock"}, relayer)
	ctx := context.Background()

	pos, err := lockSvc.LockAndEnqueue(ctx, "bank-a", "spoke-b", "0xnative", "0xmirror", "2500")
	if err != nil {
		t.Fatalf("LockAndEnqueue: %v", err)
	}
	if pos.BridgeState != podomain.BridgeStateLocking {
		t.Fatalf("expected LOCKING, got %s", pos.BridgeState)
	}
	if len(relayer.lockKeys) != 1 {
		t.Fatalf("expected 1 lock relay submit, got %d", len(relayer.lockKeys))
	}
	if got := countQueue(t, db); got != 1 {
		t.Fatalf("expected 1 queue item after lock, got %d", got)
	}

	// Relayer confirms the Hub mint: position becomes ACTIVE.
	if err := db.Model(&podomain.BridgedAssetPosition{}).
		Where("position_id = ?", pos.PositionID).
		Update("bridge_state", podomain.BridgeStateActive).Error; err != nil {
		t.Fatalf("confirm mint: %v", err)
	}

	burned, err := burnSvc.BurnAndEnqueue(ctx, pos.PositionID)
	if err != nil {
		t.Fatalf("BurnAndEnqueue: %v", err)
	}
	if burned.BridgeState != podomain.BridgeStateBurning {
		t.Fatalf("expected BURNING, got %s", burned.BridgeState)
	}
	if got := loadPosition(t, db, pos.PositionID).BridgeState; got != podomain.BridgeStateBurning {
		t.Fatalf("DB state expected BURNING, got %s", got)
	}
	if len(relayer.burnKeys) != 1 {
		t.Fatalf("expected 1 burn relay submit, got %d", len(relayer.burnKeys))
	}
	if got := countQueue(t, db); got != 2 {
		t.Fatalf("expected 2 queue items (lock+burn), got %d", got)
	}
}

// TestBridge_LockLegFails_NoPartialSettlement: when the on-chain lock reverts,
// NO position and NO queue item may be created.
func TestBridge_LockLegFails_NoPartialSettlement(t *testing.T) {
	db := newBridgeDB(t)
	relayer := &fakeRelayer{}
	lockSvc := posvc.NewBridgeLockMintService(db, &fakeLockCaller{err: errors.New("revert: insufficient reserve")}, relayer)

	_, err := lockSvc.LockAndEnqueue(context.Background(), "bank-a", "spoke-b", "0xnative", "0xmirror", "2500")
	if err == nil {
		t.Fatal("expected lock failure")
	}
	if got := countPositions(t, db); got != 0 {
		t.Errorf("no position may persist when lock fails, got %d", got)
	}
	if got := countQueue(t, db); got != 0 {
		t.Errorf("no queue item may be enqueued when lock fails, got %d", got)
	}
	if len(relayer.lockKeys) != 0 {
		t.Errorf("relayer must not be invoked when lock fails, got %d", len(relayer.lockKeys))
	}
}

// TestBridge_UnlockLegFails_StateRemainsActive: when the on-chain unlock reverts
// during burn→unlock, the position MUST stay ACTIVE (refundable / retryable) and
// no burn queue item may exist — prevents a burned-but-not-unlocked partial state.
func TestBridge_UnlockLegFails_StateRemainsActive(t *testing.T) {
	db := newBridgeDB(t)
	seedActivePosition(t, db, "pos-x", "2500")
	burnSvc := posvc.NewBridgeBurnUnlockService(db, &fakeUnlockCaller{err: errors.New("revert")}, &fakeRelayer{})

	_, err := burnSvc.BurnAndEnqueue(context.Background(), "pos-x")
	if err == nil {
		t.Fatal("expected unlock failure")
	}
	if got := loadPosition(t, db, "pos-x").BridgeState; got != podomain.BridgeStateActive {
		t.Errorf("state must remain ACTIVE after unlock failure, got %s", got)
	}
	if got := countQueue(t, db); got != 0 {
		t.Errorf("no burn queue item may exist after unlock failure, got %d", got)
	}
}

// TestBridge_NonActivePositionCannotBurn: a position not in ACTIVE state cannot be
// burned — guards against double-spend and burning a leg whose mint never settled.
func TestBridge_NonActivePositionCannotBurn(t *testing.T) {
	for _, state := range []podomain.BridgeState{
		podomain.BridgeStateLocking,
		podomain.BridgeStateBurning,
		podomain.BridgeStateBurned,
		podomain.BridgeStateReleased,
	} {
		t.Run(string(state), func(t *testing.T) {
			db := newBridgeDB(t)
			pos := &podomain.BridgedAssetPosition{
				PositionID: "pos-1", OwnerBankID: "bank-a", SpokeNetwork: "spoke-b",
				NativeAsset: "0xn", MirroredAsset: "0xm", MirroredAmount: "100", BridgeState: state,
			}
			if err := db.Create(pos).Error; err != nil {
				t.Fatalf("seed: %v", err)
			}
			caller := &fakeUnlockCaller{txHash: "0x1"}
			burnSvc := posvc.NewBridgeBurnUnlockService(db, caller, &fakeRelayer{})

			if _, err := burnSvc.BurnAndEnqueue(context.Background(), "pos-1"); err == nil {
				t.Fatal("expected rejection for non-active position")
			}
			if caller.calls != 0 {
				t.Errorf("unlock must not be called for non-active position")
			}
			if got := countQueue(t, db); got != 0 {
				t.Errorf("no queue item expected, got %d", got)
			}
		})
	}
}

// TestBridge_TimeoutRefundPath simulates the timeout/refund branch: a position
// whose mint never confirmed (stuck in LOCKING past its deadline) is refunded —
// modeled as a transition to RELEASED — and is then NOT settleable. The atomicity
// invariant holds: a refunded position cannot also be settled.
func TestBridge_TimeoutRefundPath(t *testing.T) {
	db := newBridgeDB(t)
	relayer := &fakeRelayer{}
	lockSvc := posvc.NewBridgeLockMintService(db, &fakeLockCaller{txHash: "0xlock"}, relayer)
	ctx := context.Background()

	pos, err := lockSvc.LockAndEnqueue(ctx, "bank-a", "spoke-b", "0xnative", "0xmirror", "2500")
	if err != nil {
		t.Fatalf("LockAndEnqueue: %v", err)
	}

	// Simulate the reconciler observing no Hub mint before the deadline and
	// driving the refund: LOCKING → RELEASED (funds returned on the Spoke).
	deadline := time.Now().Add(-time.Minute)
	if !time.Now().After(deadline) {
		t.Fatal("precondition: deadline should be in the past")
	}
	if err := db.Model(&podomain.BridgedAssetPosition{}).
		Where("position_id = ? AND bridge_state = ?", pos.PositionID, podomain.BridgeStateLocking).
		Update("bridge_state", podomain.BridgeStateReleased).Error; err != nil {
		t.Fatalf("refund transition: %v", err)
	}
	if got := loadPosition(t, db, pos.PositionID).BridgeState; got != podomain.BridgeStateReleased {
		t.Fatalf("expected RELEASED after refund, got %s", got)
	}

	burnSvc := posvc.NewBridgeBurnUnlockService(db, &fakeUnlockCaller{txHash: "0x1"}, relayer)
	if _, err := burnSvc.BurnAndEnqueue(ctx, pos.PositionID); err == nil {
		t.Fatal("refunded position must not be settleable (burn must fail)")
	}
}

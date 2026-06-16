// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	podmain "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
)

func TestLockAndEnqueue_Success(t *testing.T) {
	db := newTestDB(t)
	bridge := &fakeLockCaller{txHash: "0xabc"}
	relayer := &fakeLockRelayer{}
	svc := NewBridgeLockMintService(db, bridge, relayer)

	pos, err := svc.LockAndEnqueue(context.Background(), "bank-a", "spoke-a", "0xnative", "0xmirror", "1000")
	if err != nil {
		t.Fatalf("LockAndEnqueue: %v", err)
	}

	// Position created and persisted in LOCKING state — the leg-1 lock succeeded.
	if pos.BridgeState != podmain.BridgeStateLocking {
		t.Errorf("expected LOCKING, got %s", pos.BridgeState)
	}
	if pos.MirroredAmount != "1000" {
		t.Errorf("expected amount 1000, got %s", pos.MirroredAmount)
	}
	if pos.FirstAttemptAt == nil || pos.LastAttemptAt == nil {
		t.Error("expected attempt timestamps set")
	}
	if bridge.gotAmt != "1000" {
		t.Errorf("bridge got amount %s", bridge.gotAmt)
	}

	// Exactly one position and one queue item persisted.
	if n := countPositions(t, db); n != 1 {
		t.Errorf("expected 1 position, got %d", n)
	}
	if n := countQueueItems(t, db); n != 1 {
		t.Errorf("expected 1 queue item, got %d", n)
	}

	// Queue item is idempotency-keyed on positionID + txHash and PENDING.
	var item podmain.RelayerQueueItem
	if err := db.First(&item).Error; err != nil {
		t.Fatalf("load queue item: %v", err)
	}
	if item.PositionID != pos.PositionID {
		t.Errorf("queue item position mismatch")
	}
	if item.EventType != podmain.RelayerEventTypeLockMint {
		t.Errorf("expected LOCK_MINT event type, got %s", item.EventType)
	}
	if item.State != podmain.RelayerStatePending {
		t.Errorf("expected PENDING, got %s", item.State)
	}
	wantKey := "lock:" + pos.PositionID + ":0xabc"
	if item.IdempotencyKey != wantKey {
		t.Errorf("expected key %q, got %q", wantKey, item.IdempotencyKey)
	}
	if relayer.calls != 1 || relayer.keys[0] != wantKey {
		t.Errorf("relayer submit mismatch: calls=%d keys=%v", relayer.calls, relayer.keys)
	}
}

func TestLockAndEnqueue_Validation(t *testing.T) {
	tests := []struct {
		name                                              string
		owner, spoke, native, amount string
	}{
		{"missing owner", "", "spoke", "native", "1"},
		{"missing spoke", "owner", "", "native", "1"},
		{"missing native", "owner", "spoke", "", "1"},
		{"missing amount", "owner", "spoke", "native", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := newTestDB(t)
			bridge := &fakeLockCaller{txHash: "0x1"}
			svc := NewBridgeLockMintService(db, bridge, &fakeLockRelayer{})

			_, err := svc.LockAndEnqueue(context.Background(), tc.owner, tc.spoke, tc.native, "mirror", tc.amount)
			if err == nil {
				t.Fatal("expected validation error")
			}
			// Atomicity: no lock attempted, nothing persisted on bad input.
			if bridge.calls != 0 {
				t.Errorf("bridge should not be called on invalid input")
			}
			if n := countPositions(t, db); n != 0 {
				t.Errorf("expected 0 positions, got %d", n)
			}
			if n := countQueueItems(t, db); n != 0 {
				t.Errorf("expected 0 queue items, got %d", n)
			}
		})
	}
}

// When the on-chain lock (leg 1) fails, no position and no queue item may exist:
// nothing was locked, so there must be nothing to mint. No partial settlement.
func TestLockAndEnqueue_LockFailsNoPersistence(t *testing.T) {
	db := newTestDB(t)
	bridge := &fakeLockCaller{err: errors.New("revert: insufficient balance")}
	relayer := &fakeLockRelayer{}
	svc := NewBridgeLockMintService(db, bridge, relayer)

	_, err := svc.LockAndEnqueue(context.Background(), "bank-a", "spoke-a", "0xnative", "0xmirror", "1000")
	if err == nil {
		t.Fatal("expected lock error")
	}
	if !strings.Contains(err.Error(), "spoke bridge lock failed") {
		t.Errorf("unexpected error: %v", err)
	}
	if n := countPositions(t, db); n != 0 {
		t.Errorf("expected 0 positions after lock failure, got %d", n)
	}
	if n := countQueueItems(t, db); n != 0 {
		t.Errorf("expected 0 queue items after lock failure, got %d", n)
	}
	if relayer.calls != 0 {
		t.Errorf("relayer must not be called after lock failure")
	}
}

// A relayer-submit failure is non-fatal: the position and queue item are still
// persisted so the RelayerWorker can retry from the DB. Lock leg already succeeded.
func TestLockAndEnqueue_RelayerSubmitFailureIsNonFatal(t *testing.T) {
	db := newTestDB(t)
	bridge := &fakeLockCaller{txHash: "0xdef"}
	relayer := &fakeLockRelayer{err: errors.New("relayer unreachable")}
	svc := NewBridgeLockMintService(db, bridge, relayer)

	pos, err := svc.LockAndEnqueue(context.Background(), "bank-a", "spoke-a", "0xnative", "0xmirror", "1000")
	if err != nil {
		t.Fatalf("expected success despite relayer failure: %v", err)
	}
	if pos == nil {
		t.Fatal("expected position returned")
	}
	if n := countQueueItems(t, db); n != 1 {
		t.Errorf("queue item must persist for worker retry, got %d", n)
	}
}

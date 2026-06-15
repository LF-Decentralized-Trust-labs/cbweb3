// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	podmain "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"gorm.io/gorm"
)

func seedQueueItem(t *testing.T, db *gorm.DB, eventType string, state podmain.RelayerItemState, nextAt time.Time) *podmain.RelayerQueueItem {
	t.Helper()
	pos := &podmain.BridgedAssetPosition{
		PositionID:     "pos-" + eventType,
		OwnerBankID:    "bank-a",
		SpokeNetwork:   "spoke",
		NativeAsset:    "0xnative",
		MirroredAsset:  "0xmirror",
		MirroredAmount: "100",
		BridgeState:    podmain.BridgeStateLocking,
	}
	if err := db.Create(pos).Error; err != nil {
		t.Fatalf("seed position: %v", err)
	}
	item := &podmain.RelayerQueueItem{
		ItemID:         "item-" + eventType,
		IdempotencyKey: "key-" + eventType,
		EventType:      eventType,
		PositionID:     pos.PositionID,
		State:          state,
		NextAttemptAt:  nextAt,
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("seed item: %v", err)
	}
	return item
}

func loadItem(t *testing.T, db *gorm.DB, itemID string) podmain.RelayerQueueItem {
	t.Helper()
	var it podmain.RelayerQueueItem
	if err := db.Where("item_id = ?", itemID).First(&it).Error; err != nil {
		t.Fatalf("load item %s: %v", itemID, err)
	}
	return it
}

func loadPos(t *testing.T, db *gorm.DB, positionID string) podmain.BridgedAssetPosition {
	t.Helper()
	var p podmain.BridgedAssetPosition
	if err := db.Where("position_id = ?", positionID).First(&p).Error; err != nil {
		t.Fatalf("load pos %s: %v", positionID, err)
	}
	return p
}

// LOCK_MINT success: item COMPLETED and position advances LOCKING → ACTIVE.
func TestRelayerWorker_LockMintSuccess(t *testing.T) {
	db := newTestDB(t)
	item := seedQueueItem(t, db, podmain.RelayerEventTypeLockMint, podmain.RelayerStatePending, time.Now().Add(-time.Minute))
	ex := newFakeExecutor()
	w := NewRelayerWorker(db, ex, time.Millisecond, false)

	w.tick(context.Background())

	got := loadItem(t, db, item.ItemID)
	if got.State != podmain.RelayerStateCompleted {
		t.Errorf("expected COMPLETED, got %s", got.State)
	}
	if pos := loadPos(t, db, item.PositionID); pos.BridgeState != podmain.BridgeStateActive {
		t.Errorf("expected ACTIVE, got %s", pos.BridgeState)
	}
	if ex.lockCalls[item.PositionID] != 1 {
		t.Errorf("expected 1 lock call, got %d", ex.lockCalls[item.PositionID])
	}
}

// BURN_UNLOCK success in hub-only mode advances to RELEASED; full mode to BURNED.
func TestRelayerWorker_BurnUnlockSuccess(t *testing.T) {
	tests := []struct {
		name     string
		hubOnly  bool
		expected podmain.BridgeState
	}{
		{"hub-only", true, podmain.BridgeStateReleased},
		{"full", false, podmain.BridgeStateBurned},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := newTestDB(t)
			item := seedQueueItem(t, db, podmain.RelayerEventTypeBurnUnlock, podmain.RelayerStatePending, time.Now().Add(-time.Minute))
			ex := newFakeExecutor()
			w := NewRelayerWorker(db, ex, time.Millisecond, tc.hubOnly)

			w.tick(context.Background())

			if got := loadItem(t, db, item.ItemID); got.State != podmain.RelayerStateCompleted {
				t.Errorf("expected COMPLETED, got %s", got.State)
			}
			if pos := loadPos(t, db, item.PositionID); pos.BridgeState != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, pos.BridgeState)
			}
		})
	}
}

// A transient failure (below max attempts) re-schedules the item to PENDING with
// incremented attempt_count, backoff in the future, and last_error recorded.
func TestRelayerWorker_TransientFailureRetries(t *testing.T) {
	db := newTestDB(t)
	item := seedQueueItem(t, db, podmain.RelayerEventTypeLockMint, podmain.RelayerStatePending, time.Now().Add(-time.Minute))
	ex := newFakeExecutor()
	ex.lockErr[item.PositionID] = errors.New("hub mint reverted")
	w := NewRelayerWorker(db, ex, time.Millisecond, false)

	before := time.Now()
	w.tick(context.Background())

	got := loadItem(t, db, item.ItemID)
	if got.State != podmain.RelayerStatePending {
		t.Errorf("expected PENDING after transient failure, got %s", got.State)
	}
	if got.AttemptCount != 1 {
		t.Errorf("expected attempt_count 1, got %d", got.AttemptCount)
	}
	if !strings.Contains(got.LastError, "hub mint reverted") {
		t.Errorf("expected last_error recorded, got %q", got.LastError)
	}
	if !got.NextAttemptAt.After(before) {
		t.Errorf("expected backoff scheduled in the future")
	}
	// Position must NOT advance on failure — no partial settlement.
	if pos := loadPos(t, db, item.PositionID); pos.BridgeState != podmain.BridgeStateLocking {
		t.Errorf("position must remain LOCKING on failure, got %s", pos.BridgeState)
	}
}

// After maxRelayerAttempts-1 prior failures, the next failure escalates: item
// ESCALATED and position → RECONCILIATION_REQUIRED with a populated error log.
func TestRelayerWorker_ExhaustionEscalates(t *testing.T) {
	db := newTestDB(t)
	item := seedQueueItem(t, db, podmain.RelayerEventTypeLockMint, podmain.RelayerStatePending, time.Now().Add(-time.Minute))
	// Pre-set attempt count to one below the max so the next failure exhausts.
	if err := db.Model(&podmain.RelayerQueueItem{}).Where("item_id = ?", item.ItemID).
		Update("attempt_count", maxRelayerAttempts-1).Error; err != nil {
		t.Fatalf("seed attempts: %v", err)
	}
	ex := newFakeExecutor()
	ex.lockErr[item.PositionID] = errors.New("permanent failure")
	w := NewRelayerWorker(db, ex, time.Millisecond, false)

	w.tick(context.Background())

	got := loadItem(t, db, item.ItemID)
	if got.State != podmain.RelayerStateEscalated {
		t.Errorf("expected ESCALATED, got %s", got.State)
	}
	pos := loadPos(t, db, item.PositionID)
	if pos.BridgeState != podmain.BridgeStateReconciliationRequired {
		t.Errorf("expected RECONCILIATION_REQUIRED, got %s", pos.BridgeState)
	}
	if !strings.Contains(pos.RelayerErrorLog, "RELAYER_EXHAUSTED") {
		t.Errorf("expected error log populated, got %q", pos.RelayerErrorLog)
	}
}

func TestRelayerWorker_UnknownEventTypeFailsAndRetries(t *testing.T) {
	db := newTestDB(t)
	item := seedQueueItem(t, db, "WAT", podmain.RelayerStatePending, time.Now().Add(-time.Minute))
	ex := newFakeExecutor()
	w := NewRelayerWorker(db, ex, time.Millisecond, false)

	w.tick(context.Background())

	got := loadItem(t, db, item.ItemID)
	if got.AttemptCount != 1 {
		t.Errorf("expected attempt_count 1, got %d", got.AttemptCount)
	}
	if !strings.Contains(got.LastError, "unknown event_type") {
		t.Errorf("expected unknown event_type error, got %q", got.LastError)
	}
}

// Items not yet due (next_attempt_at in the future) are not processed.
func TestRelayerWorker_RespectsNextAttemptAt(t *testing.T) {
	db := newTestDB(t)
	item := seedQueueItem(t, db, podmain.RelayerEventTypeLockMint, podmain.RelayerStatePending, time.Now().Add(time.Hour))
	ex := newFakeExecutor()
	w := NewRelayerWorker(db, ex, time.Millisecond, false)

	w.tick(context.Background())

	if ex.lockCalls[item.PositionID] != 0 {
		t.Errorf("future item must not be processed")
	}
	if got := loadItem(t, db, item.ItemID); got.State != podmain.RelayerStatePending {
		t.Errorf("expected still PENDING, got %s", got.State)
	}
}

// Completed/escalated items are not picked up by the poller.
func TestRelayerWorker_SkipsTerminalStates(t *testing.T) {
	db := newTestDB(t)
	item := seedQueueItem(t, db, podmain.RelayerEventTypeLockMint, podmain.RelayerStateCompleted, time.Now().Add(-time.Hour))
	ex := newFakeExecutor()
	w := NewRelayerWorker(db, ex, time.Millisecond, false)

	w.tick(context.Background())

	if ex.lockCalls[item.PositionID] != 0 {
		t.Errorf("completed item must not be reprocessed")
	}
}

// Run loops until the context is cancelled and processes due items.
func TestRelayerWorker_RunProcessesAndStops(t *testing.T) {
	db := newTestDB(t)
	item := seedQueueItem(t, db, podmain.RelayerEventTypeLockMint, podmain.RelayerStatePending, time.Now().Add(-time.Minute))
	ex := newFakeExecutor()
	w := NewRelayerWorker(db, ex, 5*time.Millisecond, false)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()

	deadline := time.After(2 * time.Second)
	for {
		if loadItem(t, db, item.ItemID).State == podmain.RelayerStateCompleted {
			break
		}
		select {
		case <-deadline:
			cancel()
			t.Fatal("item not processed in time")
		default:
			time.Sleep(2 * time.Millisecond)
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after cancel")
	}
}

// A DB query failure in tick is logged and returns without panicking.
func TestRelayerWorker_TickQueryErrorHandled(t *testing.T) {
	db := newTestDB(t)
	// Remove the table so the poll query errors.
	if err := db.Migrator().DropTable(&podmain.RelayerQueueItem{}); err != nil {
		t.Fatalf("drop table: %v", err)
	}
	w := NewRelayerWorker(db, newFakeExecutor(), time.Millisecond, false)
	w.tick(context.Background()) // must not panic
}

// Backoff is capped at backoffCapSeconds.
func TestRelayerWorker_BackoffCapped(t *testing.T) {
	db := newTestDB(t)
	item := seedQueueItem(t, db, podmain.RelayerEventTypeLockMint, podmain.RelayerStatePending, time.Now().Add(-time.Minute))
	if err := db.Model(&podmain.RelayerQueueItem{}).Where("item_id = ?", item.ItemID).
		Update("attempt_count", 3).Error; err != nil {
		t.Fatalf("seed attempts: %v", err)
	}
	ex := newFakeExecutor()
	ex.lockErr[item.PositionID] = errors.New("fail")
	w := NewRelayerWorker(db, ex, time.Millisecond, false)

	before := time.Now()
	w.tick(context.Background())

	got := loadItem(t, db, item.ItemID)
	maxNext := before.Add(time.Duration(backoffCapSeconds+2) * time.Second)
	if got.NextAttemptAt.After(maxNext) {
		t.Errorf("backoff exceeded cap: next=%v cap=%v", got.NextAttemptAt, maxNext)
	}
}

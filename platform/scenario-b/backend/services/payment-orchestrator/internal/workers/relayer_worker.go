// Package workers provides the Relayer idempotent retry worker for Scenario B (FR-031 / FR-039 / Decision 11).
package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	podmain "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"gorm.io/gorm"
)

const (
	maxRelayerAttempts = 5
	backoffCapSeconds  = 60
)

// RelayerEventExecutor dispatches on-chain events.
type RelayerEventExecutor interface {
	SubmitLockEvent(ctx context.Context, idempotencyKey, positionID string) error
	SubmitBurnEvent(ctx context.Context, idempotencyKey, positionID string) error
}

// RelayerWorker polls the relayer_queue_items table and retries with exponential backoff.
type RelayerWorker struct {
	db                *gorm.DB
	executor          RelayerEventExecutor
	interval          time.Duration
	hubOnlyBridgeMode bool // true when spoke lock/release is skipped (local dev)
}

// NewRelayerWorker creates a RelayerWorker.
func NewRelayerWorker(db *gorm.DB, executor RelayerEventExecutor, interval time.Duration, hubOnlyBridgeMode bool) *RelayerWorker {
	return &RelayerWorker{db: db, executor: executor, interval: interval, hubOnlyBridgeMode: hubOnlyBridgeMode}
}

// Run starts the polling loop; blocks until ctx is cancelled.
func (w *RelayerWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *RelayerWorker) tick(ctx context.Context) {
	var items []podmain.RelayerQueueItem
	now := time.Now()

	if err := w.db.WithContext(ctx).
		Where("state IN ? AND next_attempt_at <= ?", []podmain.RelayerItemState{
			podmain.RelayerStatePending,
			podmain.RelayerStateInFlight,
		}, now).
		Order("next_attempt_at ASC").
		Limit(50).
		Find(&items).Error; err != nil {
		log.Printf("[RelayerWorker] db query error: %v", err)
		return
	}

	for i := range items {
		w.processItem(ctx, &items[i])
	}
}

func (w *RelayerWorker) processItem(ctx context.Context, item *podmain.RelayerQueueItem) {
	w.db.WithContext(ctx).Model(item).Update("state", podmain.RelayerStateInFlight)

	var execErr error
	switch item.EventType {
	case podmain.RelayerEventTypeLockMint:
		execErr = w.executor.SubmitLockEvent(ctx, item.IdempotencyKey, item.PositionID)
	case podmain.RelayerEventTypeBurnUnlock:
		execErr = w.executor.SubmitBurnEvent(ctx, item.IdempotencyKey, item.PositionID)
	default:
		execErr = fmt.Errorf("unknown event_type: %s", item.EventType)
	}

	if execErr == nil {
		w.db.WithContext(ctx).Model(item).Updates(map[string]interface{}{
			"state": podmain.RelayerStateCompleted,
		})
		// Advance bridge_state based on event type.
		var nextBridgeState podmain.BridgeState
		switch item.EventType {
		case podmain.RelayerEventTypeLockMint:
			nextBridgeState = podmain.BridgeStateActive
		case podmain.RelayerEventTypeBurnUnlock:
			if w.hubOnlyBridgeMode {
				nextBridgeState = podmain.BridgeStateReleased
			} else {
				nextBridgeState = podmain.BridgeStateBurned
			}
		default:
			nextBridgeState = podmain.BridgeStateReleased
		}
		w.db.WithContext(ctx).Model(&podmain.BridgedAssetPosition{}).
			Where("position_id = ?", item.PositionID).
			Update("bridge_state", nextBridgeState)
		return
	}

	newAttempts := item.AttemptCount + 1
	item.AttemptCount = newAttempts
	item.LastError = execErr.Error()

	if newAttempts >= maxRelayerAttempts {
		log.Printf("[RelayerWorker] item %s exhausted after %d attempts: %v", item.ItemID, newAttempts, execErr)
		w.escalate(ctx, item)
		return
	}

	backoffSecs := int(math.Min(float64(int(2)<<uint(newAttempts)), float64(backoffCapSeconds)))
	backoff := time.Duration(backoffSecs) * time.Second
	next := time.Now().Add(backoff)
	w.db.WithContext(ctx).Model(item).Updates(map[string]interface{}{
		"state":           podmain.RelayerStatePending,
		"attempt_count":   newAttempts,
		"last_error":      execErr.Error(),
		"next_attempt_at": next,
	})
}

func (w *RelayerWorker) escalate(ctx context.Context, item *podmain.RelayerQueueItem) {
	w.db.WithContext(ctx).Model(item).Updates(map[string]interface{}{
		"state":         podmain.RelayerStateEscalated,
		"attempt_count": item.AttemptCount,
		"last_error":    item.LastError,
	})

	errorLog, _ := json.Marshal(map[string]interface{}{
		"reason":       "RELAYER_EXHAUSTED",
		"last_error":   item.LastError,
		"attempts":     item.AttemptCount,
		"escalated_at": time.Now().Format(time.RFC3339),
	})
	w.db.WithContext(ctx).Model(&podmain.BridgedAssetPosition{}).
		Where("position_id = ?", item.PositionID).
		Updates(map[string]interface{}{
			"bridge_state":      podmain.BridgeStateReconciliationRequired,
			"relayer_error_log": string(errorLog),
		})
}

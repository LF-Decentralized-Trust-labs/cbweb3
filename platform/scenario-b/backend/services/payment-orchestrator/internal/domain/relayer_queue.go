// Package domain defines the RelayerQueueItem model for Scenario B idempotent bridging retries.
package domain

import "time"

// RelayerItemState enumerates the states of a relayer queue item (Decision 11 / FR-031 / FR-039).
type RelayerItemState string

const (
	RelayerStatePending   RelayerItemState = "PENDING"
	RelayerStateInFlight  RelayerItemState = "IN_FLIGHT"
	RelayerStateCompleted RelayerItemState = "COMPLETED"
	RelayerStateFailed    RelayerItemState = "FAILED"
	RelayerStateEscalated RelayerItemState = "ESCALATED"
)

// LockResult is returned by SpokeBridgeLockCaller after a successful lock transaction.
type LockResult struct {
	TxHash string
}

// UnlockResult is returned by SpokeBridgeUnlockCaller after a successful unlock transaction.
type UnlockResult struct {
	TxHash string
}

// RelayerEventType enumerates events the Relayer queue processes.
type RelayerEventType = string

const (
	RelayerEventTypeLockMint   RelayerEventType = "LOCK_MINT"
	RelayerEventTypeBurnUnlock RelayerEventType = "BURN_UNLOCK"
)

// RelayerItemStatePending is an alias matching the canonical PENDING constant name.
const RelayerItemStatePending = RelayerStatePending
type RelayerQueueItem struct {
	ItemID         string           `gorm:"primaryKey;column:item_id;type:varchar(64)"`
	IdempotencyKey string           `gorm:"column:idempotency_key;not null;uniqueIndex"`
	EventType      string           `gorm:"column:event_type;not null"` // LOCK_MINT | BURN_UNLOCK
	PositionID     string           `gorm:"column:position_id;not null"`
	State          RelayerItemState `gorm:"column:state;not null;default:'PENDING'"`
	AttemptCount   int              `gorm:"column:attempt_count;not null;default:0"`
	NextAttemptAt  time.Time        `gorm:"column:next_attempt_at"`
	LastError      string           `gorm:"column:last_error"`
	CreatedAt      time.Time        `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time        `gorm:"column:updated_at;autoUpdateTime"`
}

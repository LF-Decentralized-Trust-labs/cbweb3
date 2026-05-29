// Package domain defines SwapRollbackLog model for auditing automatic rollback
// attempts when swap fails after bridge-in succeeds.
//
// Feature: 009-commercial-cross-currency-swap
// Spec: specs/009-commercial-cross-currency-swap/data-model.md
package domain

import "time"

// RollbackStatus is the state of a swap rollback operation.
type RollbackStatus string

const (
	RollbackStatusPending    RollbackStatus = "PENDING"
	RollbackStatusInProgress RollbackStatus = "IN_PROGRESS"
	RollbackStatusCompleted  RollbackStatus = "COMPLETED"
	RollbackStatusFailed     RollbackStatus = "FAILED"
)

// SwapRollbackLog records rollback attempts (bridge reverso Hub→Spoke-A)
// when swap fails after successful bridge-in.
type SwapRollbackLog struct {
	RollbackID         string         `gorm:"primaryKey;column:rollback_id;type:varchar(64)"`
	SwapOperationID    string         `gorm:"column:swap_operation_id;not null"`
	BridgeInPositionID string         `gorm:"column:bridge_in_position_id;not null"`
	RollbackStatus     RollbackStatus `gorm:"column:rollback_status;not null;default:'PENDING'"`
	BurnTxHash         *string        `gorm:"column:burn_tx_hash"`
	UnlockTxHash       *string        `gorm:"column:unlock_tx_hash"`
	FailureReason      *string        `gorm:"column:failure_reason"`
	RetryCount         int            `gorm:"column:retry_count;not null;default:0"`
	CreatedAt          time.Time      `gorm:"column:created_at;autoCreateTime"`
	CompletedAt        *time.Time     `gorm:"column:completed_at"`
}

// TableName overrides GORM's default table name.
func (SwapRollbackLog) TableName() string {
	return "swap_rollback_logs"
}

// MaxRetriesExceeded returns true if retry_count >= 3 (requires manual intervention).
func (r *SwapRollbackLog) MaxRetriesExceeded() bool {
	return r.RetryCount >= 3
}

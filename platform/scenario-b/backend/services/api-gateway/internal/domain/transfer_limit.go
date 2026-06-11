// Package domain defines TransferLimit model for configurable CB transfer limits (R1-10.1).
package domain

import "time"

// TransferLimit stores a configurable daily transfer limit set by a Central Bank.
// Limits are matched by specificity: (ParticipantID+Currency) > (ParticipantID) > (Currency) > (global).
// Empty string in ParticipantID or Currency means "applies to all".
type TransferLimit struct {
	LimitID       string    `gorm:"primaryKey;column:limit_id;type:varchar(64)"`
	CentralBankID string    `gorm:"column:central_bank_id;not null;index"`
	ParticipantID string    `gorm:"column:participant_id;not null;default:''"`
	Currency      string    `gorm:"column:currency;not null;default:''"`
	MaxAmount     string    `gorm:"column:max_amount;not null"`
	IsActive      bool      `gorm:"column:is_active;not null;default:true"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName overrides GORM's default table name.
func (TransferLimit) TableName() string {
	return "transfer_limits"
}

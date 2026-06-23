// SPDX-License-Identifier: Apache-2.0

// Package domain defines TransferLimit model for configurable CB transfer limits (R1-10.1).
package domain

import "time"

// TransferLimit stores a configurable daily transfer limit set by a Central Bank.
// Limits are matched by specificity: (ParticipantID+Currency) > (ParticipantID) > (Currency) > (global).
// Empty string in ParticipantID or Currency means "applies to all".
type TransferLimit struct {
	LimitID       string    `gorm:"primaryKey;column:limit_id;type:varchar(64)"      json:"limit_id"`
	CentralBankID string    `gorm:"column:central_bank_id;not null;index"            json:"central_bank_id"`
	ParticipantID string    `gorm:"column:participant_id;not null;default:''"        json:"participant_id"`
	Currency      string    `gorm:"column:currency;not null;default:''"              json:"currency"`
	MaxAmount     string    `gorm:"column:max_amount;not null"                       json:"max_amount"`
	IsActive      bool      `gorm:"column:is_active;not null;default:true"           json:"is_active"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"                 json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime"                 json:"updated_at"`
}

// TableName overrides GORM's default table name.
func (TransferLimit) TableName() string {
	return "transfer_limits"
}

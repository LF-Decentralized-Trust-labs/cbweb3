// Package domain defines TransferVolumeLog for daily transfer volume tracking (R1-10.1).
package domain

import "time"

// TransferVolumeLog accumulates the total transfer volume per participant per currency per UTC day.
// Upserted atomically on each transfer initiation; decremented on synchronous rollback.
type TransferVolumeLog struct {
	LogID             string    `gorm:"primaryKey;column:log_id;type:varchar(64)"`
	ParticipantID     string    `gorm:"column:participant_id;not null;uniqueIndex:idx_volume_log_window"`
	Currency          string    `gorm:"column:currency;not null;uniqueIndex:idx_volume_log_window"`
	WindowDate        time.Time `gorm:"column:window_date;not null;uniqueIndex:idx_volume_log_window"`
	AccumulatedAmount string    `gorm:"column:accumulated_amount;not null;default:'0'"`
	UpdatedAt         time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName overrides GORM's default table name.
func (TransferVolumeLog) TableName() string {
	return "transfer_volume_logs"
}

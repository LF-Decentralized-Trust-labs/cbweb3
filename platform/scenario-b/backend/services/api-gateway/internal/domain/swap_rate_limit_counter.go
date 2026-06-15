// SPDX-License-Identifier: Apache-2.0

// Package domain defines SwapRateLimitCounter model for database-backed rate limiting
// (10 swaps/min, 100 swaps/hour per bank).
//
// Feature: 009-commercial-cross-currency-swap
// Spec: specs/009-commercial-cross-currency-swap/data-model.md
package domain

import "time"

// RateLimitWindow is the type of time window for rate limiting.
type RateLimitWindow string

const (
	RateLimitWindowMinute RateLimitWindow = "MINUTE"
	RateLimitWindowHour   RateLimitWindow = "HOUR"
)

// SwapRateLimitCounter tracks rate limiting counters per commercial bank
// (database-backed MVP, can migrate to Redis in production).
type SwapRateLimitCounter struct {
	ID          string          `gorm:"primaryKey;column:id;type:varchar(64)"`
	BankID      string          `gorm:"column:bank_id;not null;index"`
	WindowType  RateLimitWindow `gorm:"column:window_type;not null"`
	WindowStart time.Time       `gorm:"column:window_start;not null;index"`
	SwapCount   int             `gorm:"column:swap_count;not null;default:0"`
	LastUpdated time.Time       `gorm:"column:last_updated;autoUpdateTime"`
}

// TableName overrides GORM's default table name.
func (SwapRateLimitCounter) TableName() string {
	return "swap_rate_limit_counters"
}

// IsLimitExceeded returns true if swap_count exceeds the window limit.
func (c *SwapRateLimitCounter) IsLimitExceeded() bool {
	switch c.WindowType {
	case RateLimitWindowMinute:
		return c.SwapCount >= 10
	case RateLimitWindowHour:
		return c.SwapCount >= 100
	default:
		return false
	}
}

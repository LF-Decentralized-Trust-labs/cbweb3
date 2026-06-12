// SPDX-License-Identifier: Apache-2.0

// Package domain defines pool monitoring models for Scenario B (FR-028 / data-model.md §10-11).
package domain

import "time"

// PoolStatus represents the operational state of the AMM pool (data-model.md §7).
type PoolStatus string

const (
	PoolStatusEmpty              PoolStatus = "EMPTY"
	PoolStatusPendingCounterpart PoolStatus = "PENDING_COUNTERPART"
	PoolStatusActive             PoolStatus = "ACTIVE"
)

// PoolStateReading is a time-series snapshot of AMM pool reserves (high-cardinality, partitioned monthly).
// Extended for 005-cooperative-liquidity: fee_rate_bps, total_lp_count, pool_status.
type PoolStateReading struct {
	ReadingID    string    `gorm:"primaryKey;column:reading_id;type:varchar(64)"`
	PoolPair     string    `gorm:"column:pool_pair;not null;index"`
	ReserveA     string    `gorm:"column:reserve_a;not null"`
	ReserveB     string    `gorm:"column:reserve_b;not null"`
	CurrentRatio float64   `gorm:"column:current_ratio;not null"`
	RecordedAt   time.Time `gorm:"column:recorded_at;not null;index"`
	// Cooperative liquidity fields (005-cooperative-liquidity)
	FeeRateBps   int        `gorm:"column:fee_rate_bps;not null;default:30"`
	TotalLPCount *int       `gorm:"column:total_lp_count"`
	PoolStatus   PoolStatus `gorm:"column:pool_status;not null;default:EMPTY"`
}

// LiquidityAlert is generated when the pool ratio breaches the 70/30 threshold (FR-028 / SC-014).
// Append-only (see triggers.go).
type LiquidityAlert struct {
	AlertID       string     `gorm:"primaryKey;column:alert_id;type:varchar(64)"`
	PoolPair      string     `gorm:"column:pool_pair;not null;index"`
	BreachLevel   string     `gorm:"column:breach_level;not null"` // WARNING | BREACHED
	ObservedRatio float64    `gorm:"column:observed_ratio;not null"`
	Threshold     float64    `gorm:"column:threshold;not null"`
	TriggeredAt   time.Time  `gorm:"column:triggered_at;not null;autoCreateTime"`
	ClearedAt     *time.Time `gorm:"column:cleared_at"`
}

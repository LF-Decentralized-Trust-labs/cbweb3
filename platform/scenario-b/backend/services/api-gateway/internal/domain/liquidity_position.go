// SPDX-License-Identifier: Apache-2.0

// Package domain defines the LiquidityPosition model for Scenario B LP tracking.
package domain

import "time"

// LPStatus represents the lifecycle state of a LiquidityPosition.
type LPStatus string

const (
	LPStatusActive    LPStatus = "ACTIVE"
	LPStatusWithdrawn LPStatus = "WITHDRAWN"
)

// DepositSide indicates which token side was deposited ('A', 'B', or 'BOTH' for legacy).
type DepositSide string

const (
	DepositSideA    DepositSide = "A"
	DepositSideB    DepositSide = "B"
	DepositSideBoth DepositSide = "BOTH"
)

// LiquidityPosition records a Liquidity Provider's contribution to the pool (data-model.md §9).
// The deposit_side field distinguishes cooperative (single-sided) from legacy (dual-sided) positions.
// Cooperative positions carry shares_percentage, fee_claim_accumulated, and commit_id.
type LiquidityPosition struct {
	LPID              string     `gorm:"primaryKey;column:lp_id;type:varchar(64)"`
	ProviderBankID    string     `gorm:"column:provider_bank_id;not null;index"`
	PoolPair          string     `gorm:"column:pool_pair;not null;index"`
	TokenAContributed string     `gorm:"column:token_a_contributed;not null"`
	TokenBContributed string     `gorm:"column:token_b_contributed;not null"`
	LPShares          string     `gorm:"column:lp_shares;not null"`
	Status            LPStatus   `gorm:"column:status;not null;default:ACTIVE"`
	AddedAt           time.Time  `gorm:"column:added_at;not null;autoCreateTime"`
	WithdrawnAt       *time.Time `gorm:"column:withdrawn_at"`
	// Cooperative liquidity fields (005-cooperative-liquidity)
	DepositSide         DepositSide `gorm:"column:deposit_side;not null;default:BOTH"`
	SharesPercentage    *float64    `gorm:"column:shares_percentage"`
	FeeClaimAccumulated string      `gorm:"column:fee_claim_accumulated;not null;default:0"`
	CommitID            *string     `gorm:"column:commit_id;type:varchar(64)"`
}

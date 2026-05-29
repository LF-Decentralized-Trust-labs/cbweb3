// Package domain defines the PoolCommit model for cooperative liquidity commit-reveal (FR-001 / data-model.md §1.1).
package domain

import (
	"encoding/json"
	"time"
)

// CommitSide indicates which token side a commit covers.
type CommitSide string

const (
	CommitSideA CommitSide = "A"
	CommitSideB CommitSide = "B"
)

// CommitStatus is the lifecycle state of a PoolCommit.
type CommitStatus string

const (
	CommitStatusPending                CommitStatus = "PENDING"
	CommitStatusMatched                CommitStatus = "MATCHED"
	CommitStatusExecuted               CommitStatus = "EXECUTED"
	CommitStatusExpired                CommitStatus = "EXPIRED"
	CommitStatusReconciliationRequired CommitStatus = "RECONCILIATION_REQUIRED"
)

// PoolCommit records a liquidity provider's intent to deposit one side of a pool
// before any funds are transferred. Funds are only moved when both sides are MATCHED.
// Append-only: EXECUTED and EXPIRED commits are immutable (data-model.md §7).
type PoolCommit struct {
	CommitID            string       `gorm:"primaryKey;column:commit_id;type:varchar(64)"`
	PoolPair            string       `gorm:"column:pool_pair;not null;index"`
	ProviderID          string       `gorm:"column:provider_id;not null;index"`
	Side                CommitSide   `gorm:"column:side;not null;type:char(1)"`
	Amount              string       `gorm:"column:amount;not null"`
	Status              CommitStatus `gorm:"column:status;not null;default:PENDING"`
	CounterpartCommitID *string      `gorm:"column:counterpart_commit_id;type:varchar(64)"`
	OnChainCommitID     *[]byte      `gorm:"column:on_chain_commit_id;type:bytea"`
	CreatedAt           time.Time    `gorm:"column:created_at;not null;autoCreateTime"`
	ExpiresAt           time.Time    `gorm:"column:expires_at;not null"`
}

// TableName returns the GORM table name.
func (PoolCommit) TableName() string { return "pool_commits" }

// IsExpired reports whether the commit's TTL has elapsed.
func (c *PoolCommit) IsExpired() bool {
	return time.Now().UTC().After(c.ExpiresAt)
}

// OppositeSide returns the side that this commit is waiting for as counterpart.
func (c *PoolCommit) OppositeSide() CommitSide {
	if c.Side == CommitSideA {
		return CommitSideB
	}
	return CommitSideA
}

// PoolCommitDistribution is the JSON structure stored in LPFeeEvent.Distribution (data-model.md §1.2).
// Key: provider_id / lp_id; Value: percentage string with 4 decimal places (e.g. "60.0000").
type PoolCommitDistribution map[string]string

// MarshalJSON serialises the distribution map to a compact JSON byte slice.
func (d PoolCommitDistribution) MarshalJSON() ([]byte, error) {
	m := map[string]string(d)
	return json.Marshal(m)
}

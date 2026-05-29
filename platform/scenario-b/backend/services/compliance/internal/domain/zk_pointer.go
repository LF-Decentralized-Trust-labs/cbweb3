// Package domain defines the ComplianceZKPointer model for Scenario B ZK compliance.
package domain

import "time"

// ZKPointerState enumerates the lifecycle states of a ZK compliance pointer (FR-025 / FR-058).
type ZKPointerState string

const (
	ZKPointerValid   ZKPointerState = "VALID"
	ZKPointerExpired ZKPointerState = "EXPIRED"
	ZKPointerRevoked ZKPointerState = "REVOKED"
)

// ComplianceZKPointer stores a ZK-Pointer "fit to transact" proof for a participant.
type ComplianceZKPointer struct {
	PointerID      string         `gorm:"primaryKey;column:pointer_id;type:varchar(64)"`
	BankID         string         `gorm:"column:bank_id;not null;index"`
	TxRef          string         `gorm:"column:tx_ref"`
	CommitmentHash string         `gorm:"column:commitment_hash;not null"`
	ProofCID       string         `gorm:"column:proof_cid"`
	State          ZKPointerState `gorm:"column:state;not null;default:'VALID'"`
	ValidatedAt    *time.Time     `gorm:"column:validated_at"`
	ExpiresAt      *time.Time     `gorm:"column:expires_at"`
	CreatedAt      time.Time      `gorm:"column:created_at;autoCreateTime"`
}

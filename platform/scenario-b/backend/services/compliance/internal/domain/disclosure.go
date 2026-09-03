// SPDX-License-Identifier: Apache-2.0

// Package domain defines disclosure request models for Master Viewing Key governance (FR-034/FR-035/FR-036).
package domain

import "time"

// DisclosureState enumerates the lifecycle states of a disclosure request.
type DisclosureState string

const (
	DisclosurePending       DisclosureState = "PENDING"
	DisclosureApproved      DisclosureState = "APPROVED"
	DisclosureDenied        DisclosureState = "DENIED"
	DisclosureExpired       DisclosureState = "EXPIRED"
	DisclosureQuorumReached DisclosureState = "QUORUM_REACHED"
	// DisclosureDisclosed is reserved for future spoke-level Paladin MVK operation.
	DisclosureDisclosed DisclosureState = "DISCLOSED"
)

// DisclosureRequest formalises a regulatory investigation request for the Master Viewing Key.
// Requires quorum 2-of-3 Central Banks and expires after 72 hours (FR-034 / FR-035).
type DisclosureRequest struct {
	RequestID            string          `gorm:"primaryKey;column:request_id;type:varchar(64)"`
	RequestedByBankID    string          `gorm:"column:requested_by_bank_id;not null"`
	TargetTransactionRef string          `gorm:"column:target_transaction_ref;not null"`
	ReasonCode           string          `gorm:"column:reason_code;not null"`
	State                DisclosureState `gorm:"column:state;not null;default:'PENDING'"`
	QuorumRequired       int             `gorm:"column:quorum_required;not null;default:2"`
	QuorumReached        int             `gorm:"column:quorum_reached;not null;default:0"`
	OpenedAt             time.Time       `gorm:"column:opened_at;not null;autoCreateTime"`
	ExpiresAt            time.Time       `gorm:"column:expires_at;not null"` // opened_at + 72h
	ClosedAt             *time.Time      `gorm:"column:closed_at"`
}

// DisclosureSignature records a partial signature from a Central Bank on a disclosure request.
// Append-only (FR-048 / data-model.md §11).
type DisclosureSignature struct {
	SigID            string    `gorm:"primaryKey;column:sig_id;type:varchar(64)"`
	RequestID        string    `gorm:"column:request_id;not null;index"`
	SignerBankID     string    `gorm:"column:signer_bank_id;not null"`
	SignerWallet     string    `gorm:"column:signer_wallet;not null"`
	SignaturePayload []byte    `gorm:"column:signature_payload;type:bytea"`
	SignedAt         time.Time `gorm:"column:signed_at;not null;autoCreateTime"`
}

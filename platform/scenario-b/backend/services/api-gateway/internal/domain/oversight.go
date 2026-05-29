package domain

import "time"

// DisclosureState enumerates Master Viewing Key disclosure request states (FR-034/FR-035/FR-036).
type DisclosureState string

const (
	DisclosurePending       DisclosureState = "PENDING"
	DisclosureQuorumReached DisclosureState = "QUORUM_REACHED"
	DisclosureApproved      DisclosureState = "APPROVED" // reservado para uso futuro spoke-level
	DisclosureExpired       DisclosureState = "EXPIRED"
	DisclosureRejected      DisclosureState = "REJECTED"
)

// DisclosureRequest tracks a Master Viewing Key disclosure workflow (FR-034 / SC-023).
type DisclosureRequest struct {
	RequestID     string          `gorm:"primaryKey;column:request_id;type:varchar(64)"`
	RequestorID   string          `gorm:"column:requestor_id;not null"`
	TxRef         string          `gorm:"column:tx_ref;not null"`
	ReasonCode    string          `gorm:"column:reason_code;not null"`
	State         DisclosureState `gorm:"column:state;not null;default:'PENDING'"`
	QuorumReq     int             `gorm:"column:quorum_req;not null;default:2"`
	QuorumReached int             `gorm:"column:quorum_reached;not null;default:0"`
	OpenedAt      time.Time       `gorm:"column:opened_at;autoCreateTime"`
	ExpiresAt     time.Time       `gorm:"column:expires_at"`
	ClosedAt      *time.Time      `gorm:"column:closed_at"`
}

// DisclosureSignature records a signer's approval of a disclosure request.
type DisclosureSignature struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	RequestID string    `gorm:"column:request_id;not null;index"`
	SignerID  string    `gorm:"column:signer_id;not null"`
	SignedAt  time.Time `gorm:"column:signed_at;autoCreateTime"`
}

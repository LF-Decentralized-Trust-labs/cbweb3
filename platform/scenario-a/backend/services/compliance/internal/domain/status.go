// SPDX-License-Identifier: Apache-2.0

package domain

// ParticipantStatus represents the lifecycle state of a registered participant.
type ParticipantStatus string

const (
	StatusPending             ParticipantStatus = "PENDING"              // initial state
	StatusCredentialRequested ParticipantStatus = "CREDENTIAL_REQUESTED" // CSR + pub_key submitted, awaiting KYC review
	StatusKYCApproved         ParticipantStatus = "KYC_APPROVED"         // KYC approved, awaiting PoP + wallet bind
	StatusActive              ParticipantStatus = "ACTIVE"               // fully operational on the network
	StatusFrozen              ParticipantStatus = "FROZEN"               // temporarily suspended
	StatusRevoked             ParticipantStatus = "REVOKED"              // permanently disabled
)

// validTransitions defines the allowed state machine for participant lifecycle.
var validTransitions = map[ParticipantStatus][]ParticipantStatus{
	StatusPending:             {StatusCredentialRequested, StatusActive},
	StatusCredentialRequested: {StatusKYCApproved, StatusRevoked},
	StatusKYCApproved:         {StatusActive, StatusRevoked},
	StatusActive:              {StatusFrozen, StatusRevoked},
	StatusFrozen:              {StatusActive, StatusRevoked},
}

// IsValidTransition reports whether moving from current to next is allowed.
func IsValidTransition(current, next ParticipantStatus) bool {
	for _, allowed := range validTransitions[current] {
		if allowed == next {
			return true
		}
	}
	return false
}

// AuditCategory classifies governance actions for filtering in the portal.
type AuditCategory string

const (
	CategorySession      AuditCategory = "SESSION"
	CategoryCredential   AuditCategory = "CREDENTIAL"
	CategoryFreeze       AuditCategory = "FREEZE"
	CategoryCircuitBreaker AuditCategory = "CIRCUIT_BREAKER"
	CategoryParameter    AuditCategory = "PARAMETER"
	CategoryParticipant  AuditCategory = "PARTICIPANT"
)

// AuditSeverity indicates the importance/risk level of an audit event.
type AuditSeverity string

const (
	SeverityInfo     AuditSeverity = "INFO"
	SeverityWarning  AuditSeverity = "WARNING"
	SeverityCritical AuditSeverity = "CRITICAL"
)

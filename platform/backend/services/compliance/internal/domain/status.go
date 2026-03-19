package domain

// ParticipantStatus represents the lifecycle state of a registered participant.
type ParticipantStatus string

const (
	StatusPending  ParticipantStatus = "PENDING"  // awaiting BC approval
	StatusActive   ParticipantStatus = "ACTIVE"   // approved and operational
	StatusFrozen   ParticipantStatus = "FROZEN"   // temporarily suspended
	StatusRevoked  ParticipantStatus = "REVOKED"  // permanently disabled
)

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

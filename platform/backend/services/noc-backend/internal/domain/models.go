package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NocSpoke represents a registered blockchain network instance (spoke or hub).
type NocSpoke struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name           string         `gorm:"type:text;not null;uniqueIndex"                 json:"name"`
	CurrencyCode   string         `gorm:"type:char(3);not null"                          json:"currency_code"`
	Jurisdiction   string         `gorm:"type:text;not null"                             json:"jurisdiction"`
	Active         bool           `gorm:"not null;default:true"                          json:"active"`
	RegisteredAt   time.Time      `gorm:"not null;autoCreateTime"                        json:"registered_at"`
	DeregisteredAt *time.Time     `gorm:"default:null"                                   json:"deregistered_at,omitempty"`
	DeletedAt      gorm.DeletedAt `gorm:"index"                                          json:"-"`
}

// NocProvisionedKey holds pre-provisioned agent API key hashes bound to a spoke.
type NocProvisionedKey struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	KeyHash   string     `gorm:"type:text;not null;uniqueIndex"                 json:"key_hash"`
	SpokeID   uuid.UUID  `gorm:"type:uuid;not null;index"                       json:"spoke_id"`
	Hint      string     `gorm:"type:text"                                      json:"hint"`
	CreatedAt time.Time  `gorm:"not null;autoCreateTime"                        json:"created_at"`
	UsedAt    *time.Time `gorm:"default:null"                                   json:"used_at,omitempty"`
}

// NocAgent represents a deployed collector process for a spoke.
type NocAgent struct {
	ID                  uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	SpokeID             uuid.UUID  `gorm:"type:uuid;not null;index"                       json:"spoke_id"`
	Name                string     `gorm:"type:text;not null"                             json:"name"`
	ApiKeyHash          string     `gorm:"type:text;not null;uniqueIndex"                 json:"api_key_hash"`
	PushIntervalSeconds int        `gorm:"not null;default:15"                            json:"push_interval_seconds"`
	Status              string     `gorm:"type:text;not null;default:'REACHABLE'"         json:"status"` // REACHABLE | UNREACHABLE
	LastSeenAt          *time.Time `gorm:"default:null"                                   json:"last_seen_at,omitempty"`
	CreatedAt           time.Time  `gorm:"not null;autoCreateTime"                        json:"created_at"`
}

// NocComponent represents a monitorable infrastructure element within a spoke.
type NocComponent struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	AgentID         uuid.UUID  `gorm:"type:uuid;not null;index"                       json:"agent_id"`
	SpokeID         uuid.UUID  `gorm:"type:uuid;not null;index"                       json:"spoke_id"`
	Name            string     `gorm:"type:text;not null"                             json:"name"`
	Type            string     `gorm:"type:text;not null"                             json:"type"` // BESU | CACTI_RELAY | PALADIN | PAYMENT_ORCHESTRATOR
	Endpoint        string     `gorm:"type:text;not null"                             json:"endpoint"`
	HealthStatus    string     `gorm:"type:text;not null;default:'UNKNOWN'"           json:"health_status"` // HEALTHY | DEGRADED | OFFLINE | UNKNOWN
	LastCheckedAt   *time.Time `gorm:"default:null"                                   json:"last_checked_at,omitempty"`
	LastBlockNumber *int64     `gorm:"default:null"                                   json:"last_block_number,omitempty"`
	CreatedAt       time.Time  `gorm:"not null;autoCreateTime"                        json:"created_at"`
}

func (NocComponent) TableName() string { return "noc_components" }

// NocHealthEvent records a point-in-time health state observation.
type NocHealthEvent struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ComponentID uuid.UUID `gorm:"type:uuid;not null;index:idx_comp_occurred"     json:"component_id"`
	Status      string    `gorm:"type:text;not null"                             json:"status"`
	OccurredAt  time.Time `gorm:"not null;index:idx_comp_occurred,sort:desc"     json:"occurred_at"`
	ReceivedAt  time.Time `gorm:"not null;autoCreateTime"                        json:"received_at"`
	Diagnostic  *string   `gorm:"type:text"                                      json:"diagnostic,omitempty"`
	BlockNumber *int64    `gorm:"default:null"                                   json:"block_number,omitempty"`
}

// NocAlert records an individual failure condition.
type NocAlert struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"     json:"id"`
	ComponentID     uuid.UUID  `gorm:"type:uuid;not null;index"                           json:"component_id"`
	IncidentID      *uuid.UUID `gorm:"type:uuid;default:null"                             json:"incident_id,omitempty"`
	Severity        string     `gorm:"type:text;not null"                                 json:"severity"` // INFO | WARNING | HIGH | CRITICAL
	State           string     `gorm:"type:text;not null;default:'ACTIVE';index"          json:"state"`    // ACTIVE | RESOLVED
	Title           string     `gorm:"type:text;not null"                                 json:"title"`
	RootCauseSig    string     `gorm:"type:text;not null;index"                           json:"root_cause_sig"`
	CreatedAt       time.Time  `gorm:"not null;autoCreateTime;index:idx_alert_state_time" json:"created_at"`
	ResolvedAt      *time.Time `gorm:"default:null"                                       json:"resolved_at,omitempty"`
	AcknowledgedBy  *string    `gorm:"type:text"                                          json:"acknowledged_by,omitempty"`
}

// NocAlertDetail embeds an alert with its associated component for detail views.
type NocAlertDetail struct {
	NocAlert
	Component NocComponent `json:"component"`
}

// NocIncident groups related alerts by root cause.
type NocIncident struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	RootCauseSig string     `gorm:"type:text;not null;uniqueIndex"                 json:"root_cause_sig"`
	State        string     `gorm:"type:text;not null;default:'OPEN'"              json:"state"` // OPEN | RESOLVED
	Title        string     `gorm:"type:text;not null"                             json:"title"`
	SpokeID      *uuid.UUID `gorm:"type:uuid;default:null"                         json:"spoke_id,omitempty"`
	CreatedAt    time.Time  `gorm:"not null;autoCreateTime"                        json:"created_at"`
	ResolvedAt   *time.Time `gorm:"default:null"                                   json:"resolved_at,omitempty"`
}

// NocSloMetric stores pre-computed SLO metrics per component and time window.
type NocSloMetric struct {
	ID                   uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"      json:"id"`
	ComponentID          uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_slo_comp_window"  json:"component_id"`
	Window               string    `gorm:"type:text;not null;uniqueIndex:idx_slo_comp_window"  json:"window"` // 24h | 7d | 30d
	AvailabilityPct      float64   `gorm:"type:decimal(5,2);not null"                          json:"availability_pct"`
	P50Ms                *int      `gorm:"default:null"                                        json:"p50_ms,omitempty"`
	P95Ms                *int      `gorm:"default:null"                                        json:"p95_ms,omitempty"`
	P99Ms                *int      `gorm:"default:null"                                        json:"p99_ms,omitempty"`
	SloBreached          bool      `gorm:"not null;default:false"                              json:"slo_breached"`
	BreachDurationSeconds *int     `gorm:"default:null"                                        json:"breach_duration_seconds,omitempty"`
	ComputedAt           time.Time `gorm:"not null;autoCreateTime"                             json:"computed_at"`
}

// NocTransactionEvent records a lifecycle event of a cross-border transaction.
type NocTransactionEvent struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TxID         string    `gorm:"type:text;not null;index"                       json:"tx_id"`
	EventType    string    `gorm:"type:text;not null"                             json:"event_type"` // INITIATED | HTLC_LOCKED | RELAYED | HTLC_SETTLED | HTLC_REFUNDED | FAILED
	OccurredAt   time.Time `gorm:"not null;index:idx_tx_spoke_time,sort:desc"     json:"occurred_at"`
	SpokeID      uuid.UUID `gorm:"type:uuid;not null;index:idx_tx_spoke_time"     json:"spoke_id"`
	ParticipantID *string  `gorm:"type:text"                                      json:"participant_id,omitempty"`
	ContractID   *string   `gorm:"type:text"                                      json:"contract_id,omitempty"`
	ErrorCode    *string   `gorm:"type:text"                                      json:"error_code,omitempty"`
	ErrorMessage *string   `gorm:"type:text"                                      json:"error_message,omitempty"`
	RawPayload   *string   `gorm:"type:jsonb"                                     json:"raw_payload,omitempty"`
	ReceivedAt   time.Time `gorm:"not null;autoCreateTime"                        json:"received_at"`
}

// NocContainerLog stores rolling log lines from Docker containers.
type NocContainerLog struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement"                        json:"id"`
	ComponentID uuid.UUID `gorm:"type:uuid;not null;index:idx_log_comp_time"      json:"component_id"`
	Stream      string    `gorm:"type:text;not null"                              json:"stream"` // stdout | stderr
	LogLine     string    `gorm:"type:text;not null"                              json:"log_line"`
	OccurredAt  time.Time `gorm:"not null;index:idx_log_comp_time,sort:desc"      json:"occurred_at"`
	ReceivedAt  time.Time `gorm:"not null;autoCreateTime"                         json:"received_at"`
}

// NocAuditEntry records an operator action for immutable audit trail.
type NocAuditEntry struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Actor      string    `gorm:"type:text;not null"                            json:"actor"`       // username from JWT
	Action     string    `gorm:"type:text;not null"                            json:"action"`      // ACKNOWLEDGE_ALERT | DISMISS_ALERT
	TargetID   string    `gorm:"type:text;not null"                            json:"target_id"`   // alert ID
	TargetType string    `gorm:"type:text;not null;default:'ALERT'"            json:"target_type"`
	Detail     string    `gorm:"type:text"                                     json:"detail"`      // alert title
	CreatedAt  time.Time `gorm:"not null;autoCreateTime;index"                 json:"created_at"`
}

// NocRelayMetric is a computed (non-persisted) relay health summary derived from transaction events.
type NocRelayMetric struct {
	ID                  string    `json:"id"`                    // spoke_id
	Route               string    `json:"route"`                 // "{SpokeName} ↔ Hub"
	LatencyP50Ms        *float64  `json:"latency_p50_ms"`        // nil when no data
	LatencyP95Ms        *float64  `json:"latency_p95_ms"`        // nil when no data
	ProofSuccessRatePct float64   `json:"proof_success_rate_pct"` // 0–100
	Status              string    `json:"status"`                // HEALTHY | DEGRADED | DOWN
	UpdatedAt           time.Time `json:"updated_at"`
}

// NocLogSnapshot holds a frozen snapshot of logs captured at OFFLINE/UNKNOWN transitions.
type NocLogSnapshot struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ComponentID   uuid.UUID `gorm:"type:uuid;not null;index"                       json:"component_id"`
	TriggerStatus string    `gorm:"type:text;not null"                             json:"trigger_status"` // OFFLINE | UNKNOWN
	SnapshotAt    time.Time `gorm:"not null"                                       json:"snapshot_at"`
	LineCount     int       `gorm:"not null"                                       json:"line_count"`
	Lines         string    `gorm:"type:text;not null"                             json:"lines"` // newline-delimited
	CreatedAt     time.Time `gorm:"not null;autoCreateTime"                        json:"created_at"`
}

// Package domain defines Scenario B risk control state for the api-gateway.
package domain

import "time"

// CircuitBreakerState enumerates the circuit breaker state machine values (FR-030).
type CircuitBreakerState string

const (
	CircuitBreakerLive          CircuitBreakerState = "LIVE"
	CircuitBreakerHalted        CircuitBreakerState = "HALTED"
	CircuitBreakerResumePending CircuitBreakerState = "RESUME_PENDING"
)

// ImbalanceAlertState enumerates pool imbalance alert levels (REQ-FX-008 / FR-028).
type ImbalanceAlertState string

const (
	ImbalanceNormal  ImbalanceAlertState = "NORMAL"
	ImbalanceWarning ImbalanceAlertState = "WARNING"
	ImbalanceBreached ImbalanceAlertState = "BREACHED"
)

// ScenarioBRiskControlState tracks the operational risk control state for a pool pair (FR-030).
type ScenarioBRiskControlState struct {
	ControlID               string              `gorm:"primaryKey;column:control_id;type:varchar(64)"`
	PoolPair                string              `gorm:"column:pool_pair;not null;uniqueIndex"`
	ImbalanceThreshold      float64             `gorm:"column:imbalance_threshold;not null;default:0.70"`
	CurrentRatio            float64             `gorm:"column:current_ratio;not null;default:0"`
	ImbalanceAlertState     ImbalanceAlertState `gorm:"column:imbalance_alert_state;not null;default:'NORMAL'"`
	LastAlertAt             *time.Time          `gorm:"column:last_alert_at"`
	CircuitBreakerState     CircuitBreakerState `gorm:"column:circuit_breaker_state;not null;default:'LIVE'"`
	PauseInitiatorBankID    string              `gorm:"column:pause_initiator_bank_id"`
	PauseReasonCode         string              `gorm:"column:pause_reason_code"`
	ResumeRequestID         string              `gorm:"column:resume_request_id"`
	ResumeSignaturesRequired int                `gorm:"column:resume_signatures_required;not null;default:2"`
	UpdatedAt               time.Time           `gorm:"column:updated_at;autoUpdateTime"`
}

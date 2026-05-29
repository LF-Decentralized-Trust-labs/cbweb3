// Package services provides the Circuit Breaker service for Scenario B (FR-030 / FR-044 / SC-017 / SC-026).
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AMMCircuitBreakerCaller is the interface for on-chain circuit breaker operations.
type AMMCircuitBreakerCaller interface {
	PauseCircuitBreaker(ctx context.Context, signature []byte) (string, error)
	ProposeResume(ctx context.Context, signature []byte) (string, error)
	SignResume(ctx context.Context, requestID string, signature []byte) error
	ExecuteResume(ctx context.Context, requestID string) error
}

// CircuitBreakerService manages the asymmetric circuit breaker lifecycle (FR-030 / FR-044).
type CircuitBreakerService struct {
	db        *gorm.DB
	ammCaller AMMCircuitBreakerCaller
}

// NewCircuitBreakerService creates a CircuitBreakerService.
func NewCircuitBreakerService(db *gorm.DB, ammCaller AMMCircuitBreakerCaller) *CircuitBreakerService {
	return &CircuitBreakerService{db: db, ammCaller: ammCaller}
}

// Pause performs a 1-of-N pause of the AMM circuit breaker (FR-030).
func (s *CircuitBreakerService) Pause(ctx context.Context, pair, bankID, reasonCode string, signature []byte) error {
	txRef, err := s.ammCaller.PauseCircuitBreaker(ctx, signature)
	if err != nil {
		return fmt.Errorf("on-chain pause failed: %w", err)
	}

	sigRecord := &domain.CircuitBreakerSignature{
		SignatureID:      uuid.New().String(),
		ControlID:        pair,
		EventKind:        domain.CBEventKindPause,
		RequestID:        txRef,
		SignerBankID:     bankID,
		SignerWallet:     "",
		SignaturePayload: signature,
		OnChainTxRef:     txRef,
		SignedAt:         time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(sigRecord).Error; err != nil {
		return fmt.Errorf("persist pause signature: %w", err)
	}

	return s.db.WithContext(ctx).
		Table("scenario_b_risk_control_states").
		Where("pool_pair = ?", pair).
		Updates(map[string]interface{}{
			"circuit_breaker_state":   domain.CircuitBreakerHalted,
			"pause_initiator_bank_id": bankID,
			"pause_reason_code":       reasonCode,
		}).Error
}

// ProposeResume submits a resume proposal requiring quorum 2-of-N (FR-030 / FR-044).
func (s *CircuitBreakerService) ProposeResume(ctx context.Context, pair, bankID string, sig []byte) (string, error) {
	requestID, err := s.ammCaller.ProposeResume(ctx, sig)
	if err != nil {
		return "", fmt.Errorf("on-chain resume proposal failed: %w", err)
	}

	sigRecord := &domain.CircuitBreakerSignature{
		SignatureID:      uuid.New().String(),
		ControlID:        pair,
		EventKind:        domain.CBEventKindResume,
		RequestID:        requestID,
		SignerBankID:     bankID,
		SignerWallet:     "",
		SignaturePayload: sig,
		SignedAt:         time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(sigRecord).Error; err != nil {
		return "", fmt.Errorf("persist resume proposal: %w", err)
	}

	_ = s.db.WithContext(ctx).
		Table("scenario_b_risk_control_states").
		Where("pool_pair = ?", pair).
		Updates(map[string]interface{}{
			"circuit_breaker_state": domain.CircuitBreakerResumePending,
			"resume_request_id":     requestID,
		})

	return requestID, nil
}

// SignResume adds a signature to an existing resume request.
func (s *CircuitBreakerService) SignResume(ctx context.Context, pair, requestID, bankID string, sig []byte) error {
	if err := s.ammCaller.SignResume(ctx, requestID, sig); err != nil {
		return fmt.Errorf("on-chain sign resume failed: %w", err)
	}

	sigRecord := &domain.CircuitBreakerSignature{
		SignatureID:      uuid.New().String(),
		ControlID:        pair,
		EventKind:        domain.CBEventKindResume,
		RequestID:        requestID,
		SignerBankID:     bankID,
		SignerWallet:     "",
		SignaturePayload: sig,
		SignedAt:         time.Now(),
	}
	return s.db.WithContext(ctx).Create(sigRecord).Error
}

// ExecuteResume finalises the resume on-chain; requires quorum 2-of-N.
func (s *CircuitBreakerService) ExecuteResume(ctx context.Context, pair, requestID string) error {
	if err := s.ammCaller.ExecuteResume(ctx, requestID); err != nil {
		return fmt.Errorf("on-chain execute resume failed: %w", err)
	}
	return s.db.WithContext(ctx).
		Table("scenario_b_risk_control_states").
		Where("pool_pair = ?", pair).
		Updates(map[string]interface{}{
			"circuit_breaker_state": domain.CircuitBreakerLive,
			"resume_request_id":     "",
		}).Error
}

// CircuitBreakerStatus holds the current state of the circuit breaker for a pool pair.
type CircuitBreakerStatus struct {
	Pair            string `json:"pair"`
	State           string `json:"state"`
	PauseInitiator  string `json:"pause_initiator,omitempty"`
	PauseReason     string `json:"pause_reason,omitempty"`
	ResumeRequestID string `json:"resume_request_id,omitempty"`
}

// GetStatus returns the current circuit breaker state for a pool pair.
func (s *CircuitBreakerService) GetStatus(ctx context.Context, pair string) (*CircuitBreakerStatus, error) {
	var rcs domain.ScenarioBRiskControlState
	err := s.db.WithContext(ctx).Where("pool_pair = ?", pair).First(&rcs).Error
	if err != nil {
		// If no record exists, default to LIVE
		return &CircuitBreakerStatus{Pair: pair, State: string(domain.CircuitBreakerLive)}, nil
	}
	return &CircuitBreakerStatus{
		Pair:            rcs.PoolPair,
		State:           string(rcs.CircuitBreakerState),
		PauseInitiator:  rcs.PauseInitiatorBankID,
		PauseReason:     rcs.PauseReasonCode,
		ResumeRequestID: rcs.ResumeRequestID,
	}, nil
}

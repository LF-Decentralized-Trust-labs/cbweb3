// SPDX-License-Identifier: Apache-2.0

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
	PauseCircuitBreaker(ctx context.Context, pair string, signature []byte) (string, error)
	ProposeResume(ctx context.Context, pair string, signature []byte) (string, error)
	SignResume(ctx context.Context, pair, requestID string, signature []byte) error
	ExecuteResume(ctx context.Context, pair, requestID string) error
	// IsPaused reads the pair's on-chain circuit-breaker state (shared across all CBs).
	IsPaused(ctx context.Context, pair string) (bool, error)
	// ActiveResumeProposal returns the on-chain resume proposal (id, collected signatures,
	// required quorum) for pair; requestID is empty when none exists. Discovered from chain
	// so any CB can co-sign without holding the id from a tx receipt.
	ActiveResumeProposal(ctx context.Context, pair string) (requestID string, signatures int, quorum int, err error)
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
	txRef, err := s.ammCaller.PauseCircuitBreaker(ctx, pair, signature)
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
	requestID, err := s.ammCaller.ProposeResume(ctx, pair, sig)
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
	if err := s.ammCaller.SignResume(ctx, pair, requestID, sig); err != nil {
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
	if err := s.ammCaller.ExecuteResume(ctx, pair, requestID); err != nil {
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
	// ResumeSignatures / ResumeQuorum surface the 2-of-N progress of an in-flight resume
	// proposal (from chain), so any CB can see how many more signatures are needed.
	ResumeSignatures int `json:"resume_signatures,omitempty"`
	ResumeQuorum     int `json:"resume_quorum,omitempty"`
}

// GetStatus returns the current circuit breaker state for a pool pair.
func (s *CircuitBreakerService) GetStatus(ctx context.Context, pair string) (*CircuitBreakerStatus, error) {
	// Off-chain metadata (reason/initiator/resume request) from this gateway's projection.
	var rcs domain.ScenarioBRiskControlState
	hasRow := s.db.WithContext(ctx).Where("pool_pair = ?", pair).First(&rcs).Error == nil

	// On-chain isPaused() is the shared source of truth across all Central Banks: the DB
	// projection above is per-gateway and only reflects pause/resume done via this gateway,
	// so a pause triggered by another CB would otherwise be invisible here.
	paused, chainOK := false, false
	if s.ammCaller != nil {
		if p, cErr := s.ammCaller.IsPaused(ctx, pair); cErr == nil {
			paused, chainOK = p, true
		}
	}
	if !chainOK {
		// No on-chain read available (unconfigured caller or chain unreachable): fall back
		// to the local projection (or LIVE default).
		if hasRow {
			return &CircuitBreakerStatus{
				Pair:            rcs.PoolPair,
				State:           string(rcs.CircuitBreakerState),
				PauseInitiator:  rcs.PauseInitiatorBankID,
				PauseReason:     rcs.PauseReasonCode,
				ResumeRequestID: rcs.ResumeRequestID,
			}, nil
		}
		return &CircuitBreakerStatus{Pair: pair, State: string(domain.CircuitBreakerLive)}, nil
	}

	status := &CircuitBreakerStatus{Pair: pair, State: string(domain.CircuitBreakerLive)}
	if hasRow {
		status.PauseInitiator = rcs.PauseInitiatorBankID
		status.PauseReason = rcs.PauseReasonCode
	}
	if paused {
		status.State = string(domain.CircuitBreakerHalted)
		// Discover any in-flight resume proposal on-chain so ANY Central Bank (not only the
		// proposer) sees the request id + 2-of-N progress and can co-sign.
		if reqID, sigs, quorum, rErr := s.ammCaller.ActiveResumeProposal(ctx, pair); rErr == nil && reqID != "" {
			status.State = string(domain.CircuitBreakerResumePending)
			status.ResumeRequestID = reqID
			status.ResumeSignatures = sigs
			status.ResumeQuorum = quorum
		} else if hasRow {
			status.ResumeRequestID = rcs.ResumeRequestID
		}
	}
	return status, nil
}

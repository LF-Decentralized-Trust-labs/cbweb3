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
//
// Every write action returns the hash of the transaction it submitted, so the action stays
// traceable to a ledger record (constitution: breaker-lifecycle observability). The hash is
// empty when no chain is wired — empty means "no on-chain reference exists", never "unknown".
type AMMCircuitBreakerCaller interface {
	PauseCircuitBreaker(ctx context.Context, pair string, signature []byte) (string, error)
	// ProposeResume returns the proposal id a co-signer must submit AND the hash of the
	// transaction that created the proposal. They are different identifiers; substituting
	// one for the other breaks signing.
	ProposeResume(ctx context.Context, pair string, signature []byte) (requestID string, txHash string, err error)
	SignResume(ctx context.Context, pair, requestID string, signature []byte) (txHash string, err error)
	// ExecuteResume makes no chain call — the AMM auto-unpauses inside signResume once
	// quorum is met — so it has no transaction hash to return by design.
	ExecuteResume(ctx context.Context, pair, requestID string) error
	// LatestBreakerTxHash reads the transaction hash of the pair's most recent breaker
	// action, by ANY institution, from the AMM's own events. Empty (with a nil error) when
	// the pair has never had one. This is the shared source: a gateway's own signature table
	// only holds the actions that gateway performed, so it cannot answer "the pair's latest".
	LatestBreakerTxHash(ctx context.Context, pair string) (string, error)
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

// Pause performs a 1-of-N pause of the AMM circuit breaker (FR-030) and returns the hash of
// the on-chain pause transaction so the operator can cite it (FR-002). The hash is empty when
// no chain is wired; that is not an error and does not change the outcome of the pause.
func (s *CircuitBreakerService) Pause(ctx context.Context, pair, bankID, reasonCode string, signature []byte) (string, error) {
	txRef, err := s.ammCaller.PauseCircuitBreaker(ctx, pair, signature)
	if err != nil {
		return "", fmt.Errorf("on-chain pause failed: %w", err)
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
		return "", fmt.Errorf("persist pause signature: %w", err)
	}

	if err := s.db.WithContext(ctx).
		Table("scenario_b_risk_control_states").
		Where("pool_pair = ?", pair).
		Updates(map[string]interface{}{
			"circuit_breaker_state":   domain.CircuitBreakerHalted,
			"pause_initiator_bank_id": bankID,
			"pause_reason_code":       reasonCode,
		}).Error; err != nil {
		return "", err
	}
	return txRef, nil
}

// ProposeResume submits a resume proposal requiring quorum 2-of-N (FR-030 / FR-044). It
// returns the proposal id a co-signer must submit AND the hash of the transaction that
// created it. These are deliberately kept apart: RequestID carries the proposal id, which is
// operationally load-bearing for signing, while OnChainTxRef carries the audit reference.
func (s *CircuitBreakerService) ProposeResume(ctx context.Context, pair, bankID string, sig []byte) (string, string, error) {
	requestID, txRef, err := s.ammCaller.ProposeResume(ctx, pair, sig)
	if err != nil {
		return "", "", fmt.Errorf("on-chain resume proposal failed: %w", err)
	}

	sigRecord := &domain.CircuitBreakerSignature{
		SignatureID:      uuid.New().String(),
		ControlID:        pair,
		EventKind:        domain.CBEventKindResume,
		RequestID:        requestID,
		SignerBankID:     bankID,
		SignerWallet:     "",
		SignaturePayload: sig,
		OnChainTxRef:     txRef,
		SignedAt:         time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(sigRecord).Error; err != nil {
		return "", "", fmt.Errorf("persist resume proposal: %w", err)
	}

	_ = s.db.WithContext(ctx).
		Table("scenario_b_risk_control_states").
		Where("pool_pair = ?", pair).
		Updates(map[string]interface{}{
			"circuit_breaker_state": domain.CircuitBreakerResumePending,
			"resume_request_id":     requestID,
		})

	return requestID, txRef, nil
}

// SignResume adds a signature to an existing resume request and returns the hash of that
// signing transaction. When the signature completes quorum the AMM unpauses inside this same
// transaction, so the returned hash is also the transaction that resumed the pair.
func (s *CircuitBreakerService) SignResume(ctx context.Context, pair, requestID, bankID string, sig []byte) (string, error) {
	txRef, err := s.ammCaller.SignResume(ctx, pair, requestID, sig)
	if err != nil {
		return "", fmt.Errorf("on-chain sign resume failed: %w", err)
	}

	sigRecord := &domain.CircuitBreakerSignature{
		SignatureID:      uuid.New().String(),
		ControlID:        pair,
		EventKind:        domain.CBEventKindResume,
		RequestID:        requestID,
		SignerBankID:     bankID,
		SignerWallet:     "",
		SignaturePayload: sig,
		OnChainTxRef:     txRef,
		SignedAt:         time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(sigRecord).Error; err != nil {
		return "", err
	}
	return txRef, nil
}

// ExecuteResume finalises the resume; requires quorum 2-of-N.
//
// KNOWN LIMITATION (043-breaker-txhash-mock-docs FR-022 — documented, deliberately not
// fixed): the on-chain side of this call is a no-op, because the AMM auto-unpauses inside
// signResume once quorum is met. The caller therefore reports the resumed (LIVE) state as
// soon as the final *expected* signature lands. That is correct for the 2-of-N quorum used in
// every current environment, but it would report LIVE prematurely for any quorum greater
// than 2, where further signatures would still be outstanding. If the resume quorum is ever
// raised above 2, this must be reworked to confirm the on-chain state before reporting LIVE.
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
	// TxHash is the on-chain reference of the pair's most recent breaker action, by any
	// institution. Carrying it on status (rather than only on the write responses) is what
	// makes the value survive a page reload and appear identically to every Central Bank.
	// Omitted, never empty, when no on-chain reference exists.
	TxHash string `json:"tx_hash,omitempty"`
}

// latestTxRef returns the on-chain reference of the pair's most recent breaker action,
// regardless of which institution performed it (rule D-2). An empty string means the pair has
// no on-chain reference — either it has had no action, or the environment has no chain wired.
// A lookup failure is reported as absent rather than as an error: the reference is never
// allowed to gate a status read.
//
// The AMM's events are the source, not this gateway's signature table. The table records only
// the actions this Central Bank performed, so reading it answers "my last action": two Central
// Banks inspecting the same halted pair would then cite different transactions, while the
// portal tells the operator the reference comes from the ledger and is the same for everyone.
// The table remains the fallback for a gateway with no AMM wired, where it is the only record
// there is — and where it can only describe local actions anyway.
func (s *CircuitBreakerService) latestTxRef(ctx context.Context, pair string) string {
	if s.ammCaller != nil {
		if ref, err := s.ammCaller.LatestBreakerTxHash(ctx, pair); err == nil && ref != "" {
			return ref
		}
	}
	var sig domain.CircuitBreakerSignature
	err := s.db.WithContext(ctx).
		Where("control_id = ?", pair).
		Order("signed_at DESC").
		First(&sig).Error
	if err != nil {
		return ""
	}
	return sig.OnChainTxRef
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
	// The pair's most recent action reference is reported in every mode. It is empty in a
	// no-chain environment, which is the honest answer there.
	latestTxRef := s.latestTxRef(ctx, pair)

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
				TxHash:          latestTxRef,
			}, nil
		}
		return &CircuitBreakerStatus{
			Pair:   pair,
			State:  string(domain.CircuitBreakerLive),
			TxHash: latestTxRef,
		}, nil
	}

	status := &CircuitBreakerStatus{
		Pair:   pair,
		State:  string(domain.CircuitBreakerLive),
		TxHash: latestTxRef,
	}
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

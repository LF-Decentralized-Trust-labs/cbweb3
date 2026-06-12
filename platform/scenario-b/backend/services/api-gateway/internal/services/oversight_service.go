// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"fmt"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OversightService implements the Master Viewing Key disclosure workflow (FR-034/FR-035/FR-036).
type OversightService struct {
	db *gorm.DB
}

// NewOversightService creates a new OversightService backed by the given DB.
func NewOversightService(db *gorm.DB) *OversightService {
	return &OversightService{db: db}
}

// OpenDisclosure creates a new disclosure request.
func (s *OversightService) OpenDisclosure(ctx context.Context, txRef, requestorID, reasonCode string) (*DisclosureResult, error) {
	if txRef == "" || requestorID == "" || reasonCode == "" {
		return nil, fmt.Errorf("tx_ref, requestor_id, and reason_code are required")
	}

	now := time.Now()
	req := &domain.DisclosureRequest{
		RequestID:   uuid.NewString(),
		RequestorID: requestorID,
		TxRef:       txRef,
		ReasonCode:  reasonCode,
		State:       domain.DisclosurePending,
		QuorumReq:   2,
		OpenedAt:    now,
		ExpiresAt:   now.Add(72 * time.Hour),
	}
	if err := s.db.WithContext(ctx).Create(req).Error; err != nil {
		return nil, fmt.Errorf("create disclosure request: %w", err)
	}
	return toDisclosureResult(req), nil
}

// SignDisclosure records a signer's approval. When quorum is reached, the state transitions to APPROVED.
func (s *OversightService) SignDisclosure(ctx context.Context, requestID, signerID string) error {
	if requestID == "" || signerID == "" {
		return fmt.Errorf("request_id and signer_id are required")
	}

	var req domain.DisclosureRequest
	if err := s.db.WithContext(ctx).Where("request_id = ?", requestID).First(&req).Error; err != nil {
		return fmt.Errorf("disclosure request not found: %w", err)
	}

	if req.State != domain.DisclosurePending {
		return fmt.Errorf("disclosure request is not in PENDING state")
	}

	if time.Now().After(req.ExpiresAt) {
		s.db.WithContext(ctx).Model(&req).Update("state", domain.DisclosureExpired)
		return fmt.Errorf("disclosure request has expired")
	}

	// Check for duplicate signature
	var count int64
	s.db.WithContext(ctx).Model(&domain.DisclosureSignature{}).
		Where("request_id = ? AND signer_id = ?", requestID, signerID).Count(&count)
	if count > 0 {
		return fmt.Errorf("signer %s has already signed this request", signerID)
	}

	sig := &domain.DisclosureSignature{
		RequestID: requestID,
		SignerID:  signerID,
	}
	if err := s.db.WithContext(ctx).Create(sig).Error; err != nil {
		return fmt.Errorf("record signature: %w", err)
	}

	newQuorum := req.QuorumReached + 1
	updates := map[string]interface{}{"quorum_reached": newQuorum}
	if newQuorum >= req.QuorumReq {
		now := time.Now()
		updates["state"] = domain.DisclosureQuorumReached
		updates["closed_at"] = now
	}
	if err := s.db.WithContext(ctx).Model(&req).Updates(updates).Error; err != nil {
		return fmt.Errorf("update quorum: %w", err)
	}

	return nil
}

// GetDisclosureStatus retrieves the current status of a disclosure request.
func (s *OversightService) GetDisclosureStatus(ctx context.Context, requestID string) (*DisclosureResult, error) {
	var req domain.DisclosureRequest
	if err := s.db.WithContext(ctx).Where("request_id = ?", requestID).First(&req).Error; err != nil {
		return nil, fmt.Errorf("disclosure request not found: %w", err)
	}
	return toDisclosureResult(&req), nil
}

func toDisclosureResult(req *domain.DisclosureRequest) *DisclosureResult {
	return &DisclosureResult{
		RequestID:            req.RequestID,
		RequestedByBankID:    req.RequestorID,
		TargetTransactionRef: req.TxRef,
		ReasonCode:           req.ReasonCode,
		State:                string(req.State),
		QuorumRequired:       req.QuorumReq,
		QuorumReached:        req.QuorumReached,
		OpenedAt:             req.OpenedAt,
		ExpiresAt:            req.ExpiresAt,
		ClosedAt:             req.ClosedAt,
	}
}

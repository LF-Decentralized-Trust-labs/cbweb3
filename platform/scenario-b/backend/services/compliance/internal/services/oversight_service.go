// SPDX-License-Identifier: Apache-2.0

// Package services provides the OversightService for Master Viewing Key disclosure (FR-034/FR-035/FR-036).
//
// Architecture note: Paladin operates exclusively at the spoke level.
// Hub OversightService tracks quorum only (PENDING → QUORUM_REACHED).
// Actual MVK disclosure is a spoke-level operation via Paladin, out of scope here.
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/compliance/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OversightService manages Master Viewing Key disclosure lifecycle.
type OversightService struct {
	db *gorm.DB
}

// NewOversightService creates an OversightService.
func NewOversightService(db *gorm.DB) *OversightService {
	return &OversightService{db: db}
}

// OpenDisclosure creates a new disclosure request with a 72h expiry (FR-034).
func (s *OversightService) OpenDisclosure(ctx context.Context, txRef, requestorID, reasonCode string) (*domain.DisclosureRequest, error) {
	if txRef == "" || requestorID == "" || reasonCode == "" {
		return nil, fmt.Errorf("txRef, requestorID, and reasonCode are required")
	}

	now := time.Now()
	req := &domain.DisclosureRequest{
		RequestID:            uuid.New().String(),
		RequestedByBankID:    requestorID,
		TargetTransactionRef: txRef,
		ReasonCode:           reasonCode,
		State:                domain.DisclosurePending,
		QuorumRequired:       2,
		QuorumReached:        0,
		OpenedAt:             now,
		ExpiresAt:            now.Add(72 * time.Hour),
	}
	if err := s.db.WithContext(ctx).Create(req).Error; err != nil {
		return nil, fmt.Errorf("create disclosure request: %w", err)
	}
	return req, nil
}

// SignDisclosure adds a Central Bank signature and checks for quorum (FR-035).
func (s *OversightService) SignDisclosure(ctx context.Context, requestID, signerID string) error {
	var req domain.DisclosureRequest
	if err := s.db.WithContext(ctx).First(&req, "request_id = ?", requestID).Error; err != nil {
		return fmt.Errorf("disclosure request not found: %w", err)
	}

	if req.State == domain.DisclosureExpired || time.Now().After(req.ExpiresAt) {
		return fmt.Errorf("disclosure request has expired")
	}

	// Check that this signer hasn't already signed
	var count int64
	s.db.WithContext(ctx).Table("disclosure_signatures").
		Where("request_id = ? AND signer_bank_id = ?", requestID, signerID).
		Count(&count)
	if count > 0 {
		return fmt.Errorf("signer %s has already signed this request", signerID)
	}

	sig := &domain.DisclosureSignature{
		SigID:        uuid.New().String(),
		RequestID:    requestID,
		SignerBankID: signerID,
		SignerWallet: "",
		SignedAt:     time.Now(),
	}
	if err := s.db.WithContext(ctx).Create(sig).Error; err != nil {
		return fmt.Errorf("persist disclosure signature: %w", err)
	}

	// Increment quorum counter
	newQuorum := req.QuorumReached + 1
	updates := map[string]interface{}{"quorum_reached": newQuorum}
	if newQuorum >= req.QuorumRequired {
		updates["state"] = domain.DisclosureQuorumReached
	}
	return s.db.WithContext(ctx).Model(&req).Updates(updates).Error
}

// GetDisclosureStatus returns the current state of a disclosure request (FR-036).
func (s *OversightService) GetDisclosureStatus(ctx context.Context, requestID string) (*domain.DisclosureRequest, error) {
	var req domain.DisclosureRequest
	if err := s.db.WithContext(ctx).First(&req, "request_id = ?", requestID).Error; err != nil {
		return nil, fmt.Errorf("disclosure request not found: %w", err)
	}
	// Auto-expire check (worker handles batch; this handles on-read)
	if req.State == domain.DisclosurePending && time.Now().After(req.ExpiresAt) {
		_ = s.db.WithContext(ctx).Model(&req).Update("state", domain.DisclosureExpired).Error
		req.State = domain.DisclosureExpired
	}
	return &req, nil
}

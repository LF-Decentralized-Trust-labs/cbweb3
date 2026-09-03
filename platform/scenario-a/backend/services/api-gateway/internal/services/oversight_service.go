// SPDX-License-Identifier: Apache-2.0

// Package services provides the OversightService for the Investigation Module (FR-034/FR-035/FR-036).
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// validReasonCodes is the closed set of valid reason codes per the spec (contracts/supervisor-api.md).
var validReasonCodes = map[string]struct{}{
	"AML_ALERT":        {},
	"CFT_INVESTIGATION": {},
	"COURT_ORDER":      {},
	"REGULATORY_EXAM":  {},
}

// IsValidReasonCode returns true if the given code is a recognised disclosure reason.
func IsValidReasonCode(code string) bool {
	_, ok := validReasonCodes[code]
	return ok
}

// DisclosureResult is the service-layer DTO returned to the HTTP handler.
type DisclosureResult struct {
	RequestID            string
	RequestedByBankID    string
	TargetTransactionRef string
	ReasonCode           string
	State                string
	QuorumRequired       int
	QuorumReached        int
	OpenedAt             time.Time
	ExpiresAt            time.Time
	ClosedAt             *time.Time
}

// OversightService implements the disclosure workflow for supervisor AML/CFT investigations.
type OversightService struct {
	db *gorm.DB
}

// NewOversightService creates an OversightService backed by the given DB. db may be nil (test construction).
func NewOversightService(db *gorm.DB) *OversightService {
	return &OversightService{db: db}
}

// OpenDisclosure creates a new pending disclosure request with a 72-hour expiry.
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
	return toResult(req), nil
}

// SignDisclosure records a co-signature; transitions to QUORUM_REACHED when the second signature lands.
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
		s.db.WithContext(ctx).Model(&req).Update("state", domain.DisclosureExpired) //nolint:errcheck
		return fmt.Errorf("disclosure request has expired")
	}

	var count int64
	s.db.WithContext(ctx).Model(&domain.DisclosureSignature{}).
		Where("request_id = ? AND signer_id = ?", requestID, signerID).Count(&count)
	if count > 0 {
		return fmt.Errorf("signer %s has already signed this request", signerID)
	}

	if err := s.db.WithContext(ctx).Create(&domain.DisclosureSignature{
		RequestID: requestID,
		SignerID:  signerID,
	}).Error; err != nil {
		return fmt.Errorf("record signature: %w", err)
	}

	newQuorum := req.QuorumReached + 1
	updates := map[string]any{"quorum_reached": newQuorum}
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

// CheckDisclosureQuorum returns nil if a QUORUM_REACHED disclosure exists for txRef, error otherwise.
// Used as the gate check before the supervisor decrypt endpoint.
func (s *OversightService) CheckDisclosureQuorum(ctx context.Context, txRef string) error {
	var count int64
	s.db.WithContext(ctx).Model(&domain.DisclosureRequest{}).
		Where("tx_ref = ? AND state = ?", txRef, domain.DisclosureQuorumReached).
		Count(&count)
	if count == 0 {
		return fmt.Errorf("no approved disclosure request for tx %s", txRef)
	}
	return nil
}

// GetDisclosureStatus retrieves a disclosure request, auto-expiring PENDING requests that have passed their deadline.
func (s *OversightService) GetDisclosureStatus(ctx context.Context, requestID string) (*DisclosureResult, error) {
	var req domain.DisclosureRequest
	if err := s.db.WithContext(ctx).Where("request_id = ?", requestID).First(&req).Error; err != nil {
		return nil, fmt.Errorf("disclosure request not found: %w", err)
	}
	if req.State == domain.DisclosurePending && time.Now().After(req.ExpiresAt) {
		s.db.WithContext(ctx).Model(&req).Update("state", domain.DisclosureExpired) //nolint:errcheck
		req.State = domain.DisclosureExpired
	}
	return toResult(&req), nil
}

func toResult(req *domain.DisclosureRequest) *DisclosureResult {
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

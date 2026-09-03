// SPDX-License-Identifier: Apache-2.0

// Package app provides the concrete transfer limit repository backed by GORM (R1-10.1).
package app

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type transferLimitRepository struct {
	db *gorm.DB
}

func newTransferLimitRepository(db *gorm.DB) *transferLimitRepository {
	return &transferLimitRepository{db: db}
}

func (r *transferLimitRepository) Create(ctx context.Context, centralBankID, participantID, currency, maxAmount string) (*domain.TransferLimit, error) {
	limit := &domain.TransferLimit{
		LimitID:       uuid.NewString(),
		CentralBankID: centralBankID,
		ParticipantID: participantID,
		Currency:      currency,
		MaxAmount:     maxAmount,
		IsActive:      true,
	}
	if err := r.db.WithContext(ctx).Create(limit).Error; err != nil {
		return nil, err
	}
	return limit, nil
}

func (r *transferLimitRepository) List(ctx context.Context, centralBankID string) ([]domain.TransferLimit, error) {
	var limits []domain.TransferLimit
	err := r.db.WithContext(ctx).
		Where("central_bank_id = ? AND is_active = true", centralBankID).
		Order("created_at DESC").
		Find(&limits).Error
	return limits, err
}

func (r *transferLimitRepository) Delete(ctx context.Context, limitID, centralBankID string) error {
	result := r.db.WithContext(ctx).
		Where("limit_id = ? AND central_bank_id = ?", limitID, centralBankID).
		Delete(&domain.TransferLimit{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// FindApplicableLimit returns the most specific active limit for (centralBankID, participantID, currency).
// Specificity: (participant+currency) > (participant) > (currency) > (global).
// Returns nil, nil when no limit is configured.
func (r *transferLimitRepository) FindApplicableLimit(ctx context.Context, centralBankID, participantID, currency string) (*domain.TransferLimit, error) {
	var limit domain.TransferLimit
	err := r.db.WithContext(ctx).
		Where(
			"central_bank_id = ? AND is_active = true AND (participant_id = ? OR participant_id = '') AND (currency = ? OR currency = '')",
			centralBankID, participantID, currency,
		).
		Order("CASE WHEN participant_id != '' THEN 1 ELSE 0 END DESC, CASE WHEN currency != '' THEN 1 ELSE 0 END DESC").
		First(&limit).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &limit, nil
}

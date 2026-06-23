// SPDX-License-Identifier: Apache-2.0

// Package app provides the concrete transfer volume repository backed by GORM (R1-10.1).
package app

import (
	"context"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type transferVolumeRepository struct {
	db *gorm.DB
}

func newTransferVolumeRepository(db *gorm.DB) *transferVolumeRepository {
	return &transferVolumeRepository{db: db}
}

// Deduct adds amount (wei string) to the accumulated volume for the given window.
// Uses an upsert so concurrent initiations are handled atomically at the DB level.
func (r *transferVolumeRepository) Deduct(ctx context.Context, participantID, currency, amount string, windowDate time.Time) error {
	newRecord := domain.TransferVolumeLog{
		LogID:             uuid.NewString(),
		ParticipantID:     participantID,
		Currency:          currency,
		WindowDate:        windowDate,
		AccumulatedAmount: amount,
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "participant_id"},
				{Name: "currency"},
				{Name: "window_date"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"accumulated_amount": gorm.Expr(
					"CAST(transfer_volume_logs.accumulated_amount AS NUMERIC) + CAST(? AS NUMERIC)", amount,
				),
				"updated_at": gorm.Expr("NOW()"),
			}),
		}).
		Create(&newRecord).Error
}

// Restore subtracts amount from accumulated volume, flooring at zero.
func (r *transferVolumeRepository) Restore(ctx context.Context, participantID, currency, amount string, windowDate time.Time) error {
	return r.db.WithContext(ctx).
		Model(&domain.TransferVolumeLog{}).
		Where("participant_id = ? AND currency = ? AND window_date = ?", participantID, currency, windowDate).
		UpdateColumn(
			"accumulated_amount",
			gorm.Expr("GREATEST(CAST(accumulated_amount AS NUMERIC) - CAST(? AS NUMERIC), 0)::TEXT", amount),
		).Error
}

// GetAccumulated returns the current accumulated wei amount for the given window.
// Returns "0" when no record exists yet.
func (r *transferVolumeRepository) GetAccumulated(ctx context.Context, participantID, currency string, windowDate time.Time) (string, error) {
	var log domain.TransferVolumeLog
	err := r.db.WithContext(ctx).
		Where("participant_id = ? AND currency = ? AND window_date = ?", participantID, currency, windowDate).
		First(&log).Error
	if err == gorm.ErrRecordNotFound {
		return "0", nil
	}
	if err != nil {
		return "", err
	}
	return log.AccumulatedAmount, nil
}

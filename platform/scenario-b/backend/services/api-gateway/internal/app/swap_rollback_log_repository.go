// SPDX-License-Identifier: Apache-2.0

// Package app provides SwapRollbackLogRepository for auditing rollback attempts.
//
// Feature: 009-commercial-cross-currency-swap
// Spec: specs/009-commercial-cross-currency-swap/data-model.md
package app

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"gorm.io/gorm"
)

// swapRollbackLogRepository persists SwapRollbackLog records for audit.
type swapRollbackLogRepository struct {
	db *gorm.DB
}

// newSwapRollbackLogRepository creates a swapRollbackLogRepository backed by the given DB.
func newSwapRollbackLogRepository(db *gorm.DB) *swapRollbackLogRepository {
	return &swapRollbackLogRepository{db: db}
}

// Create inserts a new SwapRollbackLog record.
func (r *swapRollbackLogRepository) Create(ctx context.Context, log *domain.SwapRollbackLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// FindBySwapID retrieves all SwapRollbackLog records for a given swap_operation_id.
// Ordered by created_at DESC (newest first).
func (r *swapRollbackLogRepository) FindBySwapID(ctx context.Context, swapOperationID string) ([]domain.SwapRollbackLog, error) {
	var logs []domain.SwapRollbackLog
	err := r.db.WithContext(ctx).
		Where("swap_operation_id = ?", swapOperationID).
		Order("created_at DESC").
		Find(&logs).Error
	return logs, err
}

// UpdateStatus updates the rollback_status field of a SwapRollbackLog.
func (r *swapRollbackLogRepository) UpdateStatus(ctx context.Context, rollbackID string, status domain.RollbackStatus) error {
	return r.db.WithContext(ctx).
		Model(&domain.SwapRollbackLog{}).
		Where("rollback_id = ?", rollbackID).
		Update("rollback_status", status).Error
}

// IncrementRetryCount increments retry_count by 1 for a given rollback_id.
func (r *swapRollbackLogRepository) IncrementRetryCount(ctx context.Context, rollbackID string) error {
	return r.db.WithContext(ctx).
		Model(&domain.SwapRollbackLog{}).
		Where("rollback_id = ?", rollbackID).
		UpdateColumn("retry_count", gorm.Expr("retry_count + 1")).Error
}

// UpdateTxHashes updates burn_tx_hash and unlock_tx_hash fields after bridge reverso completes.
func (r *swapRollbackLogRepository) UpdateTxHashes(ctx context.Context, rollbackID, burnTxHash, unlockTxHash string) error {
	return r.db.WithContext(ctx).
		Model(&domain.SwapRollbackLog{}).
		Where("rollback_id = ?", rollbackID).
		Updates(map[string]interface{}{
			"burn_tx_hash":   burnTxHash,
			"unlock_tx_hash": unlockTxHash,
		}).Error
}

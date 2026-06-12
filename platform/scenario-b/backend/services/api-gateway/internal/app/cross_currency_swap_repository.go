// SPDX-License-Identifier: Apache-2.0

// Package app provides CrossCurrencySwapRepository for persisting swap operations.
//
// Feature: 009-commercial-cross-currency-swap
// Spec: specs/009-commercial-cross-currency-swap/data-model.md
package app

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"gorm.io/gorm"
)

// crossCurrencySwapRepository persists CrossCurrencySwapOperation records.
type crossCurrencySwapRepository struct {
	db *gorm.DB
}

// newCrossCurrencySwapRepository creates a crossCurrencySwapRepository backed by the given DB.
func newCrossCurrencySwapRepository(db *gorm.DB) *crossCurrencySwapRepository {
	return &crossCurrencySwapRepository{db: db}
}

// Create inserts a new CrossCurrencySwapOperation record.
func (r *crossCurrencySwapRepository) Create(ctx context.Context, op *domain.CrossCurrencySwapOperation) error {
	return r.db.WithContext(ctx).Create(op).Error
}

// GetByID retrieves a CrossCurrencySwapOperation by swap_id (satisfies CrossCurrencySwapRepository).
func (r *crossCurrencySwapRepository) GetByID(ctx context.Context, swapID string) (*domain.CrossCurrencySwapOperation, error) {
	var op domain.CrossCurrencySwapOperation
	err := r.db.WithContext(ctx).Where("swap_id = ?", swapID).First(&op).Error
	if err != nil {
		return nil, err
	}
	return &op, nil
}

// FindByID is an alias for GetByID for internal callers.
func (r *crossCurrencySwapRepository) FindByID(ctx context.Context, swapID string) (*domain.CrossCurrencySwapOperation, error) {
	return r.GetByID(ctx, swapID)
}

// FindByCorrelationID retrieves all CrossCurrencySwapOperation records by correlation_id.
// Useful for auditing all swap attempts linked to a single correlation ID.
func (r *crossCurrencySwapRepository) FindByCorrelationID(ctx context.Context, correlationID string) ([]domain.CrossCurrencySwapOperation, error) {
	var ops []domain.CrossCurrencySwapOperation
	err := r.db.WithContext(ctx).Where("correlation_id = ?", correlationID).Find(&ops).Error
	return ops, err
}

// UpdateStatus updates the status field of a CrossCurrencySwapOperation.
func (r *crossCurrencySwapRepository) UpdateStatus(ctx context.Context, swapID string, status domain.SwapOperationStatus) error {
	return r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("swap_id = ?", swapID).
		Update("status", status).Error
}

// UpdateBridgeInPositionID persists the bridge-in position ID after the relayer activates it.
func (r *crossCurrencySwapRepository) UpdateBridgeInPositionID(ctx context.Context, swapID string, positionID string) error {
	return r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("swap_id = ?", swapID).
		Update("bridge_in_position_id", positionID).Error
}

// UpdateAmountIn persists the actual amount_in after the AMM swap executes on Hub.
func (r *crossCurrencySwapRepository) UpdateAmountIn(ctx context.Context, swapID string, amountIn string) error {
	return r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("swap_id = ?", swapID).
		Update("amount_in", amountIn).Error
}

// UpdateSwapTxHash updates the swap_tx_hash field after swap execution on Hub.
func (r *crossCurrencySwapRepository) UpdateSwapTxHash(ctx context.Context, swapID string, txHash string) error {
	return r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("swap_id = ?", swapID).
		Update("swap_tx_hash", txHash).Error
}

// UpdateBridgeOutPositionID updates the bridge_out_position_id field after bridge-out initiated.
func (r *crossCurrencySwapRepository) UpdateBridgeOutPositionID(ctx context.Context, swapID string, positionID string) error {
	return r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("swap_id = ?", swapID).
		Update("bridge_out_position_id", positionID).Error
}

// UpdateFailureReason sets the failure_reason field when status=FAILED.
func (r *crossCurrencySwapRepository) UpdateFailureReason(ctx context.Context, swapID string, reason string) error {
	return r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("swap_id = ?", swapID).
		Updates(map[string]interface{}{
			"failure_reason": reason,
			"status":         domain.SwapStatusFailed,
		}).Error
}

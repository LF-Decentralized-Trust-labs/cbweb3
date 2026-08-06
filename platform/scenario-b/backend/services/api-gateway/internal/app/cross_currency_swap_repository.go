// SPDX-License-Identifier: Apache-2.0

// Package app provides CrossCurrencySwapRepository for persisting swap operations.
//
// Feature: 009-commercial-cross-currency-swap
// Spec: specs/009-commercial-cross-currency-swap/data-model.md
package app

import (
	"context"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
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

// ListByPayer returns a page of swap operations initiated by payerBankID, newest first,
// optionally bounded by [from, to] on created_at (nil = unbounded on that side). It also
// returns the total row count matching the filter (for pagination). limit<=0 defaults to
// 20; offset<0 becomes 0.
func (r *crossCurrencySwapRepository) ListByPayer(
	ctx context.Context,
	payerBankID string,
	from, to *time.Time,
	limit, offset int,
) ([]domain.CrossCurrencySwapOperation, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	q := r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("payer_bank_id = ?", payerBankID)
	if from != nil {
		q = q.Where("created_at >= ?", *from)
	}
	if to != nil {
		q = q.Where("created_at <= ?", *to)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var ops []domain.CrossCurrencySwapOperation
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&ops).Error; err != nil {
		return nil, 0, err
	}
	return ops, total, nil
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

// UpdateSwapResult atomically persists the swap tx hash and the realized amount_in after the
// AMM swap executes on Hub. Both columns are written in a single UPDATE so a crash cannot leave
// the record with a tx hash but a stale/empty amount_in (which now holds the real cost decoded
// from LogSwap, not the MaxAmountIn cap).
func (r *crossCurrencySwapRepository) UpdateSwapResult(ctx context.Context, swapID string, txHash string, amountIn string) error {
	return r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("swap_id = ?", swapID).
		Updates(map[string]interface{}{
			"swap_tx_hash": txHash,
			"amount_in":    amountIn,
		}).Error
}

// UpdateBridgeOutPositionID updates the bridge_out_position_id field after bridge-out initiated.
func (r *crossCurrencySwapRepository) UpdateBridgeOutPositionID(ctx context.Context, swapID string, positionID string) error {
	return r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("swap_id = ?", swapID).
		Update("bridge_out_position_id", positionID).Error
}

// UpdateResidue persists the outcome of the residue return (MaxAmountIn − realized amount_in)
// in a single UPDATE. It deliberately does not touch `status`: the payment is already final
// when the residue is handled, so a failed return records RETURN_FAILED for reconciliation
// without reopening a COMPLETED swap.
func (r *crossCurrencySwapRepository) UpdateResidue(ctx context.Context, swapID, amount, positionID string, status domain.ResidueReturnStatus) error {
	updates := map[string]interface{}{
		"residue_amount": amount,
		"residue_status": status,
	}
	if positionID != "" {
		updates["residue_position_id"] = positionID
	}
	return r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("swap_id = ?", swapID).
		Updates(updates).Error
}

// ListRetryableResidues returns swaps whose residue return failed to ENQUEUE and whose next
// attempt is due, oldest first.
//
// Only RETURN_FAILED is retryable here. RETURN_ENQUEUED already has a bridge position and is
// driven by the relayer's own queue; NONE has nothing to return; RETURN_ESCALATED gave up and
// needs a human. A NULL next_attempt_at is due immediately — that is the state a first failure
// leaves behind, since it predates any scheduling.
func (r *crossCurrencySwapRepository) ListRetryableResidues(ctx context.Context, now time.Time, limit int) ([]domain.CrossCurrencySwapOperation, error) {
	if limit <= 0 {
		limit = 50
	}
	var ops []domain.CrossCurrencySwapOperation
	err := r.db.WithContext(ctx).
		Where("residue_status = ?", domain.ResidueReturnFailed).
		// The attempt ceiling is enforced by the QUERY, not only by the status write that
		// escalates a row. Those are two separate statements: if the status write fails while
		// the counter write succeeds, the row stays RETURN_FAILED at the ceiling, and without
		// this bound it would be re-dispatched to the issuing CB on every sweep forever.
		Where("residue_attempts < ?", services.ResidueMaxAttempts()).
		Where("residue_next_attempt_at IS NULL OR residue_next_attempt_at <= ?", now).
		Order("created_at ASC").
		Limit(limit).
		Find(&ops).Error
	if err != nil {
		return nil, err
	}
	return ops, nil
}

// RecordResidueAttempt persists the attempt counter and when the next attempt becomes due.
// A nil nextAttemptAt clears the schedule, which is what a terminal outcome (enqueued or
// escalated) leaves behind.
func (r *crossCurrencySwapRepository) RecordResidueAttempt(ctx context.Context, swapID string, attempts int, nextAttemptAt *time.Time) error {
	return r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("swap_id = ?", swapID).
		Updates(map[string]interface{}{
			"residue_attempts":        attempts,
			"residue_next_attempt_at": nextAttemptAt,
		}).Error
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

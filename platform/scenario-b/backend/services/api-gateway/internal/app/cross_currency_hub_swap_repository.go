// SPDX-License-Identifier: Apache-2.0

// Package app provides the persistence for Hub AMM swaps the CB executes on behalf of a
// commercial bank (sovereign delegation of Step 2).
package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// pgUniqueViolationCode is the Postgres SQLSTATE for a unique-constraint violation.
const pgUniqueViolationCode = "23505"

// crossCurrencyHubSwapRepository records and looks up delegated Hub swaps.
type crossCurrencyHubSwapRepository struct {
	db *gorm.DB
}

// newCrossCurrencyHubSwapRepository creates the repository backed by the given DB.
func newCrossCurrencyHubSwapRepository(db *gorm.DB) *crossCurrencyHubSwapRepository {
	return &crossCurrencyHubSwapRepository{db: db}
}

// FindByBridgeInPosition returns the swap already executed against a bridge-in position,
// or (nil, nil) when none exists. A found record means the swap must not run again: the
// AMM swap is not idempotent on-chain, so a second run would spend the payer's bridged
// balance twice.
func (r *crossCurrencyHubSwapRepository) FindByBridgeInPosition(ctx context.Context, positionID string) (*services.HubSwapRecord, error) {
	var row domain.CrossCurrencyHubSwap
	err := r.db.WithContext(ctx).
		Where("bridge_in_position_id = ?", strings.TrimSpace(positionID)).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lookup hub swap for position %s: %w", positionID, err)
	}
	return toHubSwapRecord(&row), nil
}

// Record persists the executed swap. When a concurrent delegation won the race, the insert
// hits the primary-key constraint and the stored record is returned instead of an error —
// the swap is already done and the caller must report that outcome, not retry it.
//
// Only a unique violation takes this path; a transient DB error surfaces so the caller can
// react to it rather than mistake it for a duplicate.
func (r *crossCurrencyHubSwapRepository) Record(ctx context.Context, rec *services.HubSwapRecord) (*services.HubSwapRecord, error) {
	row := domain.CrossCurrencyHubSwap{
		BridgeInPositionID: strings.TrimSpace(rec.BridgeInPositionID),
		CorrelationID:      rec.CorrelationID,
		PayerBankID:        rec.PayerBankID,
		PoolPair:           rec.PoolPair,
		AmountOut:          rec.AmountOut,
		AmountIn:           rec.AmountIn,
		SwapTxHash:         rec.SwapTxHash,
	}
	err := r.db.WithContext(ctx).Create(&row).Error
	if err == nil {
		return toHubSwapRecord(&row), nil
	}
	if !isHubSwapUniqueViolation(err) {
		return nil, fmt.Errorf("record hub swap for position %s: %w", rec.BridgeInPositionID, err)
	}
	existing, findErr := r.FindByBridgeInPosition(ctx, rec.BridgeInPositionID)
	if findErr != nil {
		return nil, findErr
	}
	if existing == nil {
		// The constraint fired but no row is visible: do not paper over it.
		return nil, fmt.Errorf("record hub swap for position %s: unique violation with no stored record", rec.BridgeInPositionID)
	}
	return existing, nil
}

func toHubSwapRecord(row *domain.CrossCurrencyHubSwap) *services.HubSwapRecord {
	return &services.HubSwapRecord{
		BridgeInPositionID: row.BridgeInPositionID,
		CorrelationID:      row.CorrelationID,
		PayerBankID:        row.PayerBankID,
		PoolPair:           row.PoolPair,
		AmountOut:          row.AmountOut,
		AmountIn:           row.AmountIn,
		SwapTxHash:         row.SwapTxHash,
	}
}

// isHubSwapUniqueViolation reports whether err is a unique-constraint violation. Kept
// driver-agnostic on purpose: matching only Postgres SQLSTATE 23505 would make the
// concurrent-delegation fallback inert under SQLite (repository tests), turning a benign
// replay into a hard error.
func isHubSwapUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgUniqueViolationCode
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

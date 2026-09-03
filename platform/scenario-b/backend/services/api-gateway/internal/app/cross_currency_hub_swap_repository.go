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

// Claim reserves the position for this delegation BEFORE the trade runs.
//
// This is the guard, and its placement is the whole point. Recording the swap afterwards
// deduplicated the write and not the trade: two concurrent deliveries of one delegation both saw
// no record, both traded on the AMM — which is not idempotent on-chain — and the loser's insert
// then failed and was answered as a harmless "duplicate". The primary key can only prevent a
// second trade if the row exists before the first one.
//
// Returns (nil, true, nil) when this call owns the claim. When the row already exists it returns
// (existing, false, nil) and the caller must decide from the record's status: an EXECUTED claim is
// a replay to report, a PENDING one is a delegation still in flight, and a FAILED one needs
// reconciliation before anything else happens.
func (r *crossCurrencyHubSwapRepository) Claim(ctx context.Context, rec *services.HubSwapRecord) (*services.HubSwapRecord, bool, error) {
	row := domain.CrossCurrencyHubSwap{
		BridgeInPositionID: strings.TrimSpace(rec.BridgeInPositionID),
		CorrelationID:      rec.CorrelationID,
		PayerBankID:        rec.PayerBankID,
		PoolPair:           rec.PoolPair,
		AmountOut:          rec.AmountOut,
		Status:             services.HubSwapStatusPending,
	}
	err := r.db.WithContext(ctx).Create(&row).Error
	if err == nil {
		return nil, true, nil
	}
	if !isHubSwapUniqueViolation(err) {
		return nil, false, fmt.Errorf("claim hub swap for position %s: %w", rec.BridgeInPositionID, err)
	}
	existing, findErr := r.FindByBridgeInPosition(ctx, rec.BridgeInPositionID)
	if findErr != nil {
		return nil, false, findErr
	}
	if existing == nil {
		// The constraint fired but no row is visible: do not paper over it.
		return nil, false, fmt.Errorf("claim hub swap for position %s: unique violation with no stored record", rec.BridgeInPositionID)
	}
	return existing, false, nil
}

// Finalize writes the realized outcome onto a claim this process owns.
//
// Scoped to a PENDING row on purpose: if anything else already moved the claim, this call must not
// overwrite it, and the caller has to learn that rather than believe it recorded the trade.
func (r *crossCurrencyHubSwapRepository) Finalize(ctx context.Context, positionID, amountIn, swapTxHash string) (*services.HubSwapRecord, error) {
	id := strings.TrimSpace(positionID)
	res := r.db.WithContext(ctx).
		Model(&domain.CrossCurrencyHubSwap{}).
		Where("bridge_in_position_id = ?", id).
		Where("status = ?", services.HubSwapStatusPending).
		Updates(map[string]any{
			"amount_in":    amountIn,
			"swap_tx_hash": swapTxHash,
			"status":       services.HubSwapStatusExecuted,
		})
	if res.Error != nil {
		return nil, fmt.Errorf("finalize hub swap for position %s: %w", positionID, res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, fmt.Errorf("finalize hub swap for position %s: no PENDING claim to finalize", positionID)
	}
	return r.FindByBridgeInPosition(ctx, id)
}

// Abandon marks a claim whose trade did not complete, keeping the reason with the record.
//
// The claim is NOT deleted. A swap can fail after the transaction was broadcast, so from here
// "failed" does not mean "nothing moved"; releasing the position for a blind retry would reopen
// exactly the double-spend this claim exists to prevent. The normal flow rolls the bridge-in
// position back, so nothing legitimate is waiting on this row.
func (r *crossCurrencyHubSwapRepository) Abandon(ctx context.Context, positionID, reason string) error {
	res := r.db.WithContext(ctx).
		Model(&domain.CrossCurrencyHubSwap{}).
		Where("bridge_in_position_id = ?", strings.TrimSpace(positionID)).
		Where("status = ?", services.HubSwapStatusPending).
		Updates(map[string]any{
			"status":         services.HubSwapStatusFailed,
			"failure_reason": truncateFailureReason(reason),
		})
	if res.Error != nil {
		return fmt.Errorf("abandon hub swap claim for position %s: %w", positionID, res.Error)
	}
	return nil
}

// maxFailureReasonLen bounds what a remote error message can write into the record. An RPC error
// carries arbitrary text, and the column is not a log.
const maxFailureReasonLen = 500

func truncateFailureReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if len(reason) > maxFailureReasonLen {
		return reason[:maxFailureReasonLen]
	}
	return reason
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
		Status:             hubSwapStatusOf(row),
		FailureReason:      row.FailureReason,
	}
}

// hubSwapStatusOf reads the status a row means, not just the column.
//
// Rows written before the claim existed have an empty status and a transaction hash. Reporting them
// as PENDING would make every past swap look like a delegation in flight and refuse a legitimate
// replay answer, so a recorded hash decides.
func hubSwapStatusOf(row *domain.CrossCurrencyHubSwap) string {
	if row.Status != "" {
		return row.Status
	}
	if strings.TrimSpace(row.SwapTxHash) != "" {
		return services.HubSwapStatusExecuted
	}
	return services.HubSwapStatusPending
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

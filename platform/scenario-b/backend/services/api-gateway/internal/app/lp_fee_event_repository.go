// Package app provides the LPFeeEventRepository for fee distribution audit (FR-005 / data-model.md §1.2).
package app

import (
	"context"
	"time"

	apidomain "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"gorm.io/gorm"
)

// LPFeeEventRepository handles persistence for LPFeeEvent records.
// All writes are append-only — no UPDATE or DELETE (FR-012).
type LPFeeEventRepository struct {
	db *gorm.DB
}

// NewLPFeeEventRepository creates an LPFeeEventRepository backed by the given DB.
func NewLPFeeEventRepository(db *gorm.DB) *LPFeeEventRepository {
	return &LPFeeEventRepository{db: db}
}

// Create inserts a new LPFeeEvent. Returns an error on constraint violations.
func (r *LPFeeEventRepository) Create(ctx context.Context, event *apidomain.LPFeeEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

// SumFeesByLPID returns the total fee_amount_a and fee_amount_b accumulated for a
// given lp_id since the provided timestamp by scanning the distribution JSONB column.
// The query uses Postgres jsonb operator to extract the LP's percentage and applies it
// to fee_amount_a / fee_amount_b at aggregation time.
// Returns (sumA string, sumB string, error).
func (r *LPFeeEventRepository) SumFeesByLPID(ctx context.Context, lpID string, since time.Time) (string, string, error) {
	type result struct {
		SumA string
		SumB string
	}
	var res result
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			COALESCE(SUM(
				(fee_amount_a::numeric * (distribution->>?)::numeric / 100)
			)::text, '0') AS sum_a,
			COALESCE(SUM(
				(fee_amount_b::numeric * (distribution->>?)::numeric / 100)
			)::text, '0') AS sum_b
		FROM lp_fee_events
		WHERE distribution ?? ?
		  AND created_at >= ?
	`, lpID, lpID, lpID, since).Scan(&res).Error
	if err != nil {
		return "0", "0", err
	}
	return res.SumA, res.SumB, nil
}

// FindBySwapOrderID returns the LPFeeEvent for a given swap_order_id (for idempotency checks).
func (r *LPFeeEventRepository) FindBySwapOrderID(ctx context.Context, orderID string) (*apidomain.LPFeeEvent, error) {
	var event apidomain.LPFeeEvent
	err := r.db.WithContext(ctx).
		Where("swap_order_id = ?", orderID).
		First(&event).Error
	if err != nil {
		return nil, err
	}
	return &event, nil
}

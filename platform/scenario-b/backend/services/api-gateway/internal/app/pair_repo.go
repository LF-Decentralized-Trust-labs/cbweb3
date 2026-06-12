// SPDX-License-Identifier: Apache-2.0

// Package app provides the PairRepository for pair_proposals persistence (D11 — 005-cooperative-liquidity).
package app

import (
	"context"
	"time"

	apidomain "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"gorm.io/gorm"
)

// PairRepository handles persistence for PairProposal records.
// Rows are created on propose and updated on confirm; the pair_id primary key ensures idempotency.
type PairRepository struct {
	db *gorm.DB
}

// NewPairRepository creates a PairRepository backed by the given DB.
func NewPairRepository(db *gorm.DB) *PairRepository {
	return &PairRepository{db: db}
}

// Create inserts a new PairProposal in PROPOSED status.
func (r *PairRepository) Create(ctx context.Context, p *apidomain.PairProposal) error {
	return r.db.WithContext(ctx).Create(p).Error
}

// FindByPairID returns the PairProposal for the given pair_id, or gorm.ErrRecordNotFound.
func (r *PairRepository) FindByPairID(ctx context.Context, pairID string) (*apidomain.PairProposal, error) {
	var p apidomain.PairProposal
	if err := r.db.WithContext(ctx).First(&p, "pair_id = ?", pairID).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// Activate transitions a PairProposal from PROPOSED to ACTIVE, recording the confirmerCB and confirmed_at timestamp.
// Returns gorm.ErrRecordNotFound when no PROPOSED row with the given pairID exists, allowing callers to
// detect the cross-gateway case (pair proposed via another gateway) and sync from on-chain.
func (r *PairRepository) Activate(ctx context.Context, pairID, confirmerCB string, confirmedAt time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&apidomain.PairProposal{}).
		Where("pair_id = ? AND status = ?", pairID, apidomain.PairStatusProposed).
		Updates(map[string]interface{}{
			"status":       apidomain.PairStatusActive,
			"confirmer_cb": confirmerCB,
			"confirmed_at": confirmedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListActive returns all PairProposal rows in ACTIVE status, used by GET /api/v2/amm/pairs.
func (r *PairRepository) ListActive(ctx context.Context) ([]apidomain.PairProposal, error) {
	var pairs []apidomain.PairProposal
	if err := r.db.WithContext(ctx).
		Where("status = ?", apidomain.PairStatusActive).
		Order("proposed_at asc").
		Find(&pairs).Error; err != nil {
		return nil, err
	}
	return pairs, nil
}

// ListAll returns all PairProposal rows regardless of status, for audit or admin use.
func (r *PairRepository) ListAll(ctx context.Context) ([]apidomain.PairProposal, error) {
	var pairs []apidomain.PairProposal
	if err := r.db.WithContext(ctx).Order("proposed_at asc").Find(&pairs).Error; err != nil {
		return nil, err
	}
	return pairs, nil
}

// FindByTokenPair returns an existing PROPOSED or ACTIVE pair for the given token combination,
// checking both orders (tokenA/tokenB and tokenB/tokenA) to prevent duplicate pair proposals (FR-011).
// Returns gorm.ErrRecordNotFound when no matching pair exists.
func (r *PairRepository) FindByTokenPair(ctx context.Context, tokenA, tokenB string) (*apidomain.PairProposal, error) {
	var p apidomain.PairProposal
	err := r.db.WithContext(ctx).
		Where(
			"((token_a_address = ? AND token_b_address = ?) OR (token_a_address = ? AND token_b_address = ?)) AND status IN ?",
			tokenA, tokenB, tokenB, tokenA, []string{apidomain.PairStatusProposed, apidomain.PairStatusActive},
		).
		First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

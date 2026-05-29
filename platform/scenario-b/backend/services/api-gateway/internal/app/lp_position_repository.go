// Package app provides the LPPositionRepository for sovereign CB liquidity (007-bridge-based-cb-liquidity).
package app

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"gorm.io/gorm"
)

// lpPositionRepository persists LiquidityPosition records for the sovereign flow.
type lpPositionRepository struct {
	db *gorm.DB
}

// newLPPositionRepository creates an lpPositionRepository backed by the given DB.
func newLPPositionRepository(db *gorm.DB) *lpPositionRepository {
	return &lpPositionRepository{db: db}
}

// Create inserts a new LiquidityPosition record.
func (r *lpPositionRepository) Create(ctx context.Context, pos *domain.LiquidityPosition) error {
	return r.db.WithContext(ctx).Create(pos).Error
}

// FindByPoolPair returns all ACTIVE LiquidityPosition records for a given pool pair.
// Returns positions ordered by added_at DESC (newest first).
func (r *lpPositionRepository) FindByPoolPair(ctx context.Context, poolPair string) ([]domain.LiquidityPosition, error) {
	var positions []domain.LiquidityPosition
	err := r.db.WithContext(ctx).
		Where("pool_pair = ? AND status = ?", poolPair, domain.LPStatusActive).
		Order("added_at DESC").
		Find(&positions).Error
	return positions, err
}

// FindByProviderAndPoolPair returns all ACTIVE LiquidityPosition records for a specific provider and pool pair.
func (r *lpPositionRepository) FindByProviderAndPoolPair(ctx context.Context, providerID, poolPair string) ([]domain.LiquidityPosition, error) {
	var positions []domain.LiquidityPosition
	err := r.db.WithContext(ctx).
		Where("provider_bank_id = ? AND pool_pair = ? AND status = ?", providerID, poolPair, domain.LPStatusActive).
		Order("added_at DESC").
		Find(&positions).Error
	return positions, err
}

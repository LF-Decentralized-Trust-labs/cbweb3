// Package app provides SwapQuoteRepository for persisting swap quotes with TTL.
//
// Feature: 009-commercial-cross-currency-swap
// Spec: specs/009-commercial-cross-currency-swap/data-model.md
package app

import (
	"context"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"gorm.io/gorm"
)

// swapQuoteRepository persists SwapQuote records with timestamp-based expiry.
type swapQuoteRepository struct {
	db *gorm.DB
}

// newSwapQuoteRepository creates a swapQuoteRepository backed by the given DB.
func newSwapQuoteRepository(db *gorm.DB) *swapQuoteRepository {
	return &swapQuoteRepository{db: db}
}

// NewSwapQuoteRepository creates a SwapQuoteRepository (exported for cleanup jobs).
func NewSwapQuoteRepository(db *gorm.DB) *swapQuoteRepository {
	return newSwapQuoteRepository(db)
}

// Create inserts a new SwapQuote record.
func (r *swapQuoteRepository) Create(ctx context.Context, quote *domain.SwapQuote) error {
	return r.db.WithContext(ctx).Create(quote).Error
}

// FindByID retrieves a SwapQuote by quote_id.
func (r *swapQuoteRepository) FindByID(ctx context.Context, quoteID string) (*domain.SwapQuote, error) {
	var quote domain.SwapQuote
	err := r.db.WithContext(ctx).Where("quote_id = ?", quoteID).First(&quote).Error
	if err != nil {
		return nil, err
	}
	return &quote, nil
}

// DeleteExpired deletes SwapQuote records where created_at < cutoffTime.
// Used by background cleanup job to prevent quote accumulation.
// Cutoff is typically NOW() - 1 hour.
func (r *swapQuoteRepository) DeleteExpired(ctx context.Context, cutoffTime time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("created_at < ?", cutoffTime).
		Delete(&domain.SwapQuote{})
	return result.RowsAffected, result.Error
}

// Package app provides RateLimitCounterRepository for database-backed rate limiting.
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

// rateLimitCounterRepository persists SwapRateLimitCounter records for rate limiting.
type rateLimitCounterRepository struct {
	db *gorm.DB
}

// newRateLimitCounterRepository creates a rateLimitCounterRepository backed by the given DB.
func newRateLimitCounterRepository(db *gorm.DB) *rateLimitCounterRepository {
	return &rateLimitCounterRepository{db: db}
}

// IncrementCounter increments swap_count for a given (bank_id, window_type, window_start) tuple.
// Creates record if not exists (upsert semantics).
func (r *rateLimitCounterRepository) IncrementCounter(ctx context.Context, bankID string, windowType domain.RateLimitWindow, windowStart time.Time) error {
	// Try to increment existing counter
	result := r.db.WithContext(ctx).
		Model(&domain.SwapRateLimitCounter{}).
		Where("bank_id = ? AND window_type = ? AND window_start = ?", bankID, windowType, windowStart).
		UpdateColumn("swap_count", gorm.Expr("swap_count + 1"))

	if result.Error != nil {
		return result.Error
	}

	// If no record found, create new one
	if result.RowsAffected == 0 {
		counter := &domain.SwapRateLimitCounter{
			ID:          generateID(), // TODO: Use proper UUID generator from shared package
			BankID:      bankID,
			WindowType:  windowType,
			WindowStart: windowStart,
			SwapCount:   1,
		}
		return r.db.WithContext(ctx).Create(counter).Error
	}

	return nil
}

// CheckLimit verifies if the current swap_count for a given (bank_id, window_type, window_start)
// is below the limit (10/min, 100/hour).
// Returns (currentCount, limitExceeded, error).
func (r *rateLimitCounterRepository) CheckLimit(ctx context.Context, bankID string, windowType domain.RateLimitWindow, windowStart time.Time) (int, bool, error) {
	var counter domain.SwapRateLimitCounter
	err := r.db.WithContext(ctx).
		Where("bank_id = ? AND window_type = ? AND window_start = ?", bankID, windowType, windowStart).
		First(&counter).Error

	if err == gorm.ErrRecordNotFound {
		// No record yet, count is 0
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}

	limitExceeded := counter.IsLimitExceeded()
	return counter.SwapCount, limitExceeded, nil
}

// DeleteExpired deletes SwapRateLimitCounter records where window_start < cutoffTime.
// Used by background cleanup job to prevent counter accumulation.
// Cutoff is typically NOW() - 2 hours.
func (r *rateLimitCounterRepository) DeleteExpired(ctx context.Context, cutoffTime time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("window_start < ?", cutoffTime).
		Delete(&domain.SwapRateLimitCounter{})
	return result.RowsAffected, result.Error
}

// generateID is a placeholder for UUID generation (should use shared utility).
func generateID() string {
	// TODO: Replace with proper UUID generator from shared package
	return "temp-id-" + time.Now().Format("20060102150405.000000000")
}

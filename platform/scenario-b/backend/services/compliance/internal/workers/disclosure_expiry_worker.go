// SPDX-License-Identifier: Apache-2.0

// Package workers provides background workers for Scenario B compliance operations.
package workers

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"
)

// DisclosureExpiryWorker polls for pending DisclosureRequests past their expiry time
// and transitions them to EXPIRED (FR-035 / SC-027). The worker is idempotent.
type DisclosureExpiryWorker struct {
	db       *gorm.DB
	interval time.Duration
}

// NewDisclosureExpiryWorker creates a DisclosureExpiryWorker with the given poll interval.
func NewDisclosureExpiryWorker(db *gorm.DB, interval time.Duration) *DisclosureExpiryWorker {
	return &DisclosureExpiryWorker{db: db, interval: interval}
}

// Start runs the expiry loop until the context is cancelled.
func (w *DisclosureExpiryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.expireStale(ctx); err != nil {
				log.Printf("[disclosure_expiry] error: %v", err)
			}
		}
	}
}

func (w *DisclosureExpiryWorker) expireStale(ctx context.Context) error {
	result := w.db.WithContext(ctx).
		Table("disclosure_requests").
		Where("state = 'PENDING' AND expires_at < ?", time.Now()).
		Updates(map[string]interface{}{"state": "EXPIRED"})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		log.Printf("[disclosure_expiry] expired %d disclosure request(s)", result.RowsAffected)
	}
	return nil
}

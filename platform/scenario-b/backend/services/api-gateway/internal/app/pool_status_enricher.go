// SPDX-License-Identifier: Apache-2.0

// Package app provides the GORM-backed PoolStatusEnricher for T018 (005-cooperative-liquidity).
// Supplies total_lp_count and pending_commits[] to GET /api/v2/amm/pool/:pair/status.
package app

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"gorm.io/gorm"
)

// poolStatusEnricher implements services.PoolStatusEnricher using GORM (T018).
type poolStatusEnricher struct {
	db *gorm.DB
}

// newPoolStatusEnricher creates a poolStatusEnricher for the given database connection.
func newPoolStatusEnricher(db *gorm.DB) *poolStatusEnricher {
	return &poolStatusEnricher{db: db}
}

// CountActiveLPs returns the number of ACTIVE LiquidityPosition records for a pool pair.
func (e *poolStatusEnricher) CountActiveLPs(ctx context.Context, poolPair string) (int, error) {
	var count int64
	if err := e.db.WithContext(ctx).Model(&domain.LiquidityPosition{}).
		Where("pool_pair = ? AND status = ?", poolPair, domain.LPStatusActive).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

// ListPendingCommits returns PENDING PoolCommit records for a pool pair as lightweight summaries.
func (e *poolStatusEnricher) ListPendingCommits(ctx context.Context, poolPair string) ([]services.PendingCommitSummary, error) {
	var commits []domain.PoolCommit
	if err := e.db.WithContext(ctx).
		Where("pool_pair = ? AND status = ?", poolPair, domain.CommitStatusPending).
		Order("created_at ASC").
		Find(&commits).Error; err != nil {
		return nil, err
	}

	out := make([]services.PendingCommitSummary, 0, len(commits))
	for _, c := range commits {
		out = append(out, services.PendingCommitSummary{
			CommitID:   c.CommitID,
			ProviderID: c.ProviderID,
			Side:       string(c.Side),
			Amount:     c.Amount,
			ExpiresAt:  c.ExpiresAt,
		})
	}
	return out, nil
}

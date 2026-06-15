// SPDX-License-Identifier: Apache-2.0

// Package services provides the Liquidity Monitor service for Scenario B (FR-028 / FR-037 / SC-014).
// Performance gate: polling cadence p95 <= 15s (SC-023).
package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PoolStatusQuerier is the minimal interface the monitor needs to read pool state.
type PoolStatusQuerier interface {
	GetPoolStatus(ctx context.Context, pair string) (*PoolStatusResponse, error)
}

// LiquidityMonitorService polls pool reserves and persists readings + alerts.
type LiquidityMonitorService struct {
	db          *gorm.DB
	poolService PoolStatusQuerier
	pairs       []string
	interval    time.Duration
}

// NewLiquidityMonitorService creates a LiquidityMonitorService.
// pairs is the list of pool pairs to monitor. interval is the polling cadence.
func NewLiquidityMonitorService(db *gorm.DB, poolService PoolStatusQuerier, pairs []string, interval time.Duration) *LiquidityMonitorService {
	return &LiquidityMonitorService{
		db:          db,
		poolService: poolService,
		pairs:       pairs,
		interval:    interval,
	}
}

// Start runs the monitor loop until the context is cancelled.
func (s *LiquidityMonitorService) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, pair := range s.pairs {
				if err := s.poll(ctx, pair); err != nil {
					log.Printf("[liquidity_monitor] poll error pair=%s err=%v", pair, err)
				}
			}
		}
	}
}

func (s *LiquidityMonitorService) poll(ctx context.Context, pair string) error {
	status, err := s.poolService.GetPoolStatus(ctx, pair)
	if err != nil {
		return fmt.Errorf("get pool status: %w", err)
	}

	now := time.Now()
	reading := &domain.PoolStateReading{
		ReadingID:    uuid.New().String(),
		PoolPair:     pair,
		ReserveA:     status.ReserveA,
		ReserveB:     status.ReserveB,
		CurrentRatio: status.CurrentRatio,
		RecordedAt:   now,
	}
	if err := s.db.WithContext(ctx).Create(reading).Error; err != nil {
		log.Printf("[liquidity_monitor] persist reading err=%v", err)
	}

	if status.ImbalanceFlag {
		alert := &domain.LiquidityAlert{
			AlertID:       uuid.New().String(),
			PoolPair:      pair,
			BreachLevel:   "BREACHED",
			ObservedRatio: status.CurrentRatio,
			Threshold:     ImbalanceThreshold,
			TriggeredAt:   now,
		}
		if err := s.db.WithContext(ctx).Create(alert).Error; err != nil {
			log.Printf("[liquidity_monitor] persist alert err=%v", err)
		}
		log.Printf("[liquidity_monitor] ALERT pair=%s ratio=%.4f threshold=%.2f", pair, status.CurrentRatio, ImbalanceThreshold)
	} else {
		log.Printf("[liquidity_monitor] OK pair=%s ratio=%.4f", pair, status.CurrentRatio)
	}
	return nil
}

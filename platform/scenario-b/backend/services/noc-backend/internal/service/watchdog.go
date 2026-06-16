// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/backend/services/noc-backend/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/backend/services/noc-backend/internal/repository"
)

const retentionDays = 90

// WatchdogService periodically marks agents and their components as unreachable/unknown
// when no push has been received within the expected grace window.
type WatchdogService struct {
	agents     *repository.AgentsRepository
	components *repository.ComponentsRepository
	db         *gorm.DB
	multiplier int
}

// NewWatchdogService creates a WatchdogService.
func NewWatchdogService(
	agents *repository.AgentsRepository,
	components *repository.ComponentsRepository,
	db *gorm.DB,
	multiplier int,
) *WatchdogService {
	return &WatchdogService{
		agents:     agents,
		components: components,
		db:         db,
		multiplier: multiplier,
	}
}

// Start launches the watchdog ticker in the background. Cancel ctx to stop.
func (w *WatchdogService) Start(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.tick(ctx)
			}
		}
	}()
}

func (w *WatchdogService) tick(ctx context.Context) {
	// Find agents that have gone stale
	var staleAgents []domain.NocAgent
	w.db.Raw(`
		SELECT * FROM noc_agents
		WHERE status = 'REACHABLE'
		  AND last_seen_at IS NOT NULL
		  AND last_seen_at < NOW() - (push_interval_seconds * ? * INTERVAL '1 second')`,
		w.multiplier,
	).Scan(&staleAgents)

	for _, agent := range staleAgents {
		if err := w.components.MarkComponentsUnknownForAgent(agent.ID); err != nil {
			log.Printf("watchdog: marking components unknown for agent %s: %v", agent.ID, err)
		}
	}

	if err := w.agents.MarkStaleAgentsUnreachable(w.multiplier); err != nil {
		log.Printf("watchdog: marking stale agents unreachable: %v", err)
	}
}

// RetentionWorker runs a daily goroutine purging old rows.
type RetentionWorker struct {
	db *gorm.DB
}

// NewRetentionWorker creates a RetentionWorker.
func NewRetentionWorker(db *gorm.DB) *RetentionWorker {
	return &RetentionWorker{db: db}
}

// Start launches the daily retention purge in the background.
func (w *RetentionWorker) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.purge()
			}
		}
	}()
}

func (w *RetentionWorker) purge() {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	tables := []any{
		&domain.NocHealthEvent{},
		&domain.NocTransactionEvent{},
		&domain.NocContainerLog{},
		&domain.NocLogSnapshot{},
	}
	for _, model := range tables {
		if err := w.db.Where("created_at < ?", cutoff).Delete(model).Error; err != nil {
			log.Printf("retention: purge %T: %v", model, err)
		}
	}
	log.Printf("retention: purged records older than %s", cutoff.Format(time.RFC3339))
}

// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/repository"
)

const retentionDays = 90

// blindSpotMarker flips a stale agent's components to UNKNOWN, returning them with their
// previous status. Satisfied by *repository.ComponentsRepository.
type blindSpotMarker interface {
	MarkComponentsUnknownForAgent(agentID uuid.UUID) ([]domain.NocComponent, error)
}

// healthEvaluator raises/resolves alerts for a component health transition. Satisfied by
// *AlertService.
type healthEvaluator interface {
	Evaluate(ctx context.Context, comp *domain.NocComponent, newStatus, prevStatus string)
}

// WatchdogService periodically marks agents and their components as unreachable/unknown
// when no push has been received within the expected grace window, and alerts on the
// resulting blind spot.
type WatchdogService struct {
	agents     *repository.AgentsRepository
	components blindSpotMarker
	alerts     healthEvaluator
	db         *gorm.DB
	multiplier int
}

// NewWatchdogService creates a WatchdogService.
func NewWatchdogService(
	agents *repository.AgentsRepository,
	components blindSpotMarker,
	alerts healthEvaluator,
	db *gorm.DB,
	multiplier int,
) *WatchdogService {
	return &WatchdogService{
		agents:     agents,
		components: components,
		alerts:     alerts,
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
	// Every agent that has gone silent past its grace window — including ones already
	// marked UNREACHABLE. Filtering on status = 'REACHABLE' would match only on the first
	// tick (the same tick flips the agent to UNREACHABLE), so a blind spot would be
	// evaluated once and never again: dismissing its alert while the agent stayed dead
	// would leave the NOC silently blind, the very failure this guards against.
	var staleAgents []domain.NocAgent
	w.db.Raw(`
		SELECT * FROM noc_agents
		WHERE last_seen_at IS NOT NULL
		  AND last_seen_at < NOW() - (push_interval_seconds * ? * INTERVAL '1 second')`,
		w.multiplier,
	).Scan(&staleAgents)

	for _, agent := range staleAgents {
		w.markBlindSpot(ctx, agent.ID)
	}

	if err := w.agents.MarkStaleAgentsUnreachable(w.multiplier); err != nil {
		log.Printf("watchdog: marking stale agents unreachable: %v", err)
	}
}

// markBlindSpot flips a stale agent's components to UNKNOWN and evaluates each transition
// so the loss of visibility raises an alert.
//
// Without the evaluation, an agent dying with everything healthy left the portal showing
// UNKNOWN components and NO alert at all: the NOC went blind silently, which is the one
// failure an operations centre must never keep to itself. Evaluate is called on every tick
// while the agent stays stale (not only on the first flip), so dismissing the alert of a
// still-blind spot re-raises it, exactly like a component that stays OFFLINE — its own
// "already has an ACTIVE alert" guard is what keeps this from repeating every tick.
func (w *WatchdogService) markBlindSpot(ctx context.Context, agentID uuid.UUID) {
	affected, err := w.components.MarkComponentsUnknownForAgent(agentID)
	if err != nil {
		log.Printf("watchdog: marking components unknown for agent %s: %v", agentID, err)
		return
	}
	for i := range affected {
		comp := affected[i]
		prevStatus := comp.HealthStatus // as stored before the flip
		comp.HealthStatus = "UNKNOWN"
		w.alerts.Evaluate(ctx, &comp, "UNKNOWN", prevStatus)
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

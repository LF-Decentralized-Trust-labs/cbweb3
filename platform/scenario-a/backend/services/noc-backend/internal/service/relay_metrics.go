// SPDX-License-Identifier: Apache-2.0

package service

import (
	"math"
	"sort"
	"time"

	"gorm.io/gorm"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/domain"
)

// RelayMetricsService computes relay health metrics from transaction events.
type RelayMetricsService struct {
	db *gorm.DB
}

// NewRelayMetricsService creates a RelayMetricsService.
func NewRelayMetricsService(db *gorm.DB) *RelayMetricsService {
	return &RelayMetricsService{db: db}
}

type txEventRow struct {
	SpokeID    string
	SpokeName  string
	TxID       string
	EventType  string
	OccurredAt time.Time
}

// ComputeRelayMetrics derives relay health metrics per spoke from transaction events
// within the given time window, joining with spoke names.
func (s *RelayMetricsService) ComputeRelayMetrics(window time.Duration) ([]domain.NocRelayMetric, error) {
	since := time.Now().Add(-window)

	var rows []txEventRow
	err := s.db.Raw(`
		SELECT
			te.spoke_id::text  AS spoke_id,
			sp.name            AS spoke_name,
			te.tx_id,
			te.event_type,
			te.occurred_at
		FROM noc_transaction_events te
		JOIN noc_spokes sp ON sp.id = te.spoke_id
		WHERE te.occurred_at >= ?
		  AND te.event_type IN ('INITIATED','RELAYED')
		ORDER BY te.spoke_id, te.tx_id, te.occurred_at
	`, since).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	// Group rows by spoke → tx → events
	type spokeData struct {
		name      string
		initiated map[string]time.Time // tx_id → occurred_at
		relayed   map[string]time.Time
		lastSeen  time.Time
	}
	spokes := map[string]*spokeData{}

	for _, r := range rows {
		sd, ok := spokes[r.SpokeID]
		if !ok {
			sd = &spokeData{
				name:      r.SpokeName,
				initiated: map[string]time.Time{},
				relayed:   map[string]time.Time{},
			}
			spokes[r.SpokeID] = sd
		}
		if r.OccurredAt.After(sd.lastSeen) {
			sd.lastSeen = r.OccurredAt
		}
		switch r.EventType {
		case "INITIATED":
			sd.initiated[r.TxID] = r.OccurredAt
		case "RELAYED":
			sd.relayed[r.TxID] = r.OccurredAt
		}
	}

	// Also include spokes that have no transaction events (status = DOWN, no data)
	// so the UI doesn't silently omit inactive spokes.
	var allSpokes []domain.NocSpoke
	if err := s.db.Where("active = true").Find(&allSpokes).Error; err != nil {
		return nil, err
	}
	for _, sp := range allSpokes {
		id := sp.ID.String()
		if _, exists := spokes[id]; !exists {
			spokes[id] = &spokeData{
				name:      sp.Name,
				initiated: map[string]time.Time{},
				relayed:   map[string]time.Time{},
				lastSeen:  sp.RegisteredAt,
			}
		}
	}

	metrics := make([]domain.NocRelayMetric, 0, len(spokes))

	for spokeID, sd := range spokes {
		totalInitiated := len(sd.initiated)
		var deltas []float64

		for txID, initAt := range sd.initiated {
			if relayAt, ok := sd.relayed[txID]; ok {
				deltaMs := float64(relayAt.Sub(initAt).Milliseconds())
				if deltaMs >= 0 {
					deltas = append(deltas, deltaMs)
				}
			}
		}

		relayedCount := len(sd.relayed)
		successRate := 0.0
		if totalInitiated > 0 {
			successRate = float64(relayedCount) / float64(totalInitiated) * 100
		}

		var p50, p95 *float64
		if len(deltas) > 0 {
			sort.Float64s(deltas)
			p50val := percentile(deltas, 50)
			p95val := percentile(deltas, 95)
			p50 = &p50val
			p95 = &p95val
		}

		status := classifyRelayStatus(successRate, p95, totalInitiated)

		updatedAt := sd.lastSeen
		if updatedAt.IsZero() {
			updatedAt = time.Now()
		}

		metrics = append(metrics, domain.NocRelayMetric{
			ID:                  spokeID,
			Route:               sd.name,
			LatencyP50Ms:        p50,
			LatencyP95Ms:        p95,
			ProofSuccessRatePct: math.Round(successRate*100) / 100,
			Status:              status,
			UpdatedAt:           updatedAt,
		})
	}

	// Sort by route name for stable output
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].Route < metrics[j].Route
	})

	return metrics, nil
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := (p / 100) * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	return sorted[lo] + (idx-float64(lo))*(sorted[hi]-sorted[lo])
}

func classifyRelayStatus(successRate float64, p95Ms *float64, totalInitiated int) string {
	if totalInitiated == 0 {
		return "DOWN"
	}
	p95val := 0.0
	if p95Ms != nil {
		p95val = *p95Ms
	}
	if successRate >= 95 && p95val < 300 {
		return "HEALTHY"
	}
	if successRate >= 80 || (p95val >= 300 && p95val < 600) {
		return "DEGRADED"
	}
	return "DOWN"
}

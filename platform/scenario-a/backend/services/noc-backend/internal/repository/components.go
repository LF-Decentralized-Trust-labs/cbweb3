// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/domain"
)

const logRollingBuffer = 1000
const logSnapshotLines = 500

// ComponentsRepository provides operations on NocComponent, NocHealthEvent,
// NocContainerLog, and NocLogSnapshot.
type ComponentsRepository struct {
	db *gorm.DB
}

// NewComponentsRepository creates a new ComponentsRepository.
func NewComponentsRepository(db *gorm.DB) *ComponentsRepository {
	return &ComponentsRepository{db: db}
}

// UpsertComponent creates or updates a component identified by (agent_id, name).
func (r *ComponentsRepository) UpsertComponent(c *domain.NocComponent) error {
	err := r.db.
		Where(domain.NocComponent{AgentID: c.AgentID, Name: c.Name}).
		Assign(domain.NocComponent{
			SpokeID:         c.SpokeID,
			Type:            c.Type,
			Endpoint:        c.Endpoint,
			HealthStatus:    c.HealthStatus,
			LastCheckedAt:   c.LastCheckedAt,
			LastBlockNumber: c.LastBlockNumber,
		}).
		FirstOrCreate(c).Error
	if err != nil {
		return fmt.Errorf("components: upsert: %w", err)
	}
	return nil
}

// ListBySpoke returns all components for a spoke, with optional agent filter.
func (r *ComponentsRepository) ListBySpoke(spokeID uuid.UUID, agentID *uuid.UUID) ([]domain.NocComponent, error) {
	q := r.db.Where("spoke_id = ?", spokeID)
	if agentID != nil {
		q = q.Where("agent_id = ?", *agentID)
	}
	var components []domain.NocComponent
	if err := q.Find(&components).Error; err != nil {
		return nil, fmt.Errorf("components: list-by-spoke: %w", err)
	}
	return components, nil
}

// GetByID returns a component by primary key.
func (r *ComponentsRepository) GetByID(id uuid.UUID) (*domain.NocComponent, error) {
	var c domain.NocComponent
	if err := r.db.First(&c, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("components: get: %w", err)
	}
	return &c, nil
}

// UpdateHealthStatus sets the health_status and related fields of a component.
func (r *ComponentsRepository) UpdateHealthStatus(id uuid.UUID, status string, blockNumber *int64, checkedAt time.Time) error {
	updates := map[string]any{
		"health_status":   status,
		"last_checked_at": checkedAt,
	}
	if blockNumber != nil {
		updates["last_block_number"] = *blockNumber
	}
	return r.db.Model(&domain.NocComponent{}).Where("id = ?", id).Updates(updates).Error
}

// RecordHealthEvent inserts a NocHealthEvent row.
func (r *ComponentsRepository) RecordHealthEvent(e *domain.NocHealthEvent) error {
	if err := r.db.Create(e).Error; err != nil {
		return fmt.Errorf("components: record-health-event: %w", err)
	}
	return nil
}

// BulkRecordHealthEvents inserts multiple health events.
func (r *ComponentsRepository) BulkRecordHealthEvents(events []domain.NocHealthEvent) error {
	if len(events) == 0 {
		return nil
	}
	return r.db.Create(&events).Error
}

// GetLastHealthStatus returns the most recent HealthStatus for a component.
func (r *ComponentsRepository) GetLastHealthStatus(componentID uuid.UUID) (string, error) {
	var event domain.NocHealthEvent
	err := r.db.
		Where("component_id = ?", componentID).
		Order("occurred_at DESC").
		First(&event).Error
	if err == gorm.ErrRecordNotFound {
		return "UNKNOWN", nil
	}
	if err != nil {
		return "", err
	}
	return event.Status, nil
}

// AppendLogs inserts log lines and enforces the rolling buffer limit.
func (r *ComponentsRepository) AppendLogs(componentID uuid.UUID, lines []domain.NocContainerLog) error {
	if len(lines) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&lines).Error; err != nil {
			return err
		}
		// Trim to rolling buffer: delete oldest rows exceeding the limit
		return tx.Exec(`
			DELETE FROM noc_container_logs
			WHERE component_id = ?
			  AND id NOT IN (
				SELECT id FROM noc_container_logs
				WHERE component_id = ?
				ORDER BY id DESC
				LIMIT ?
			)`, componentID, componentID, logRollingBuffer,
		).Error
	})
}

// GetRecentLogs returns the most recent n log lines for a component.
func (r *ComponentsRepository) GetRecentLogs(componentID uuid.UUID, n int) ([]domain.NocContainerLog, error) {
	var logs []domain.NocContainerLog
	err := r.db.
		Where("component_id = ?", componentID).
		Order("id DESC").
		Limit(n).
		Find(&logs).Error
	if err != nil {
		return nil, fmt.Errorf("components: get-logs: %w", err)
	}
	// Reverse so oldest first
	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}
	return logs, nil
}

// CaptureLogSnapshot saves a frozen 500-line log snapshot.
func (r *ComponentsRepository) CaptureLogSnapshot(componentID uuid.UUID, triggerStatus string) error {
	logs, err := r.GetRecentLogs(componentID, logSnapshotLines)
	if err != nil || len(logs) == 0 {
		return err
	}
	lines := make([]byte, 0, 4096)
	for _, l := range logs {
		lines = append(lines, []byte(l.LogLine+"\n")...)
	}
	snap := &domain.NocLogSnapshot{
		ComponentID:   componentID,
		TriggerStatus: triggerStatus,
		SnapshotAt:    time.Now(),
		LineCount:     len(logs),
		Lines:         string(lines),
	}
	return r.db.Create(snap).Error
}

// MarkComponentsUnknownForAgent transitions an unreachable agent's components to UNKNOWN
// and returns them as they were BEFORE the update, so the caller can evaluate the
// transition and alert on the blind spot. Returning the previous status is the point: an
// agent that stopped pushing means the platform no longer knows the health of everything
// it was watching, and that has to be visible as an alert rather than a quiet status flip.
func (r *ComponentsRepository) MarkComponentsUnknownForAgent(agentID uuid.UUID) ([]domain.NocComponent, error) {
	var before []domain.NocComponent
	if err := r.db.Where("agent_id = ?", agentID).Find(&before).Error; err != nil {
		return nil, err
	}
	if len(before) == 0 {
		return nil, nil
	}
	if err := r.db.
		Model(&domain.NocComponent{}).
		Where("agent_id = ? AND health_status != 'UNKNOWN'", agentID).
		Update("health_status", "UNKNOWN").Error; err != nil {
		return nil, err
	}

	// Record the transition in the health history, exactly as a push would. This is not
	// bookkeeping: prevStatus on the push path is read from the health events, so without
	// an UNKNOWN event the first healthy push after recovery sees HEALTHY→HEALTHY, takes
	// the no-change path, and never resolves the alerts raised for the blind spot.
	now := time.Now()
	events := make([]domain.NocHealthEvent, 0, len(before))
	for _, comp := range before {
		if comp.HealthStatus == "UNKNOWN" {
			continue // already recorded on an earlier tick
		}
		events = append(events, domain.NocHealthEvent{
			ComponentID: comp.ID,
			Status:      "UNKNOWN",
			OccurredAt:  now,
			ReceivedAt:  now,
		})
	}
	if len(events) > 0 {
		if err := r.db.Create(&events).Error; err != nil {
			return before, fmt.Errorf("components: recording unknown transition: %w", err)
		}
	}
	return before, nil
}

// HealthHistory returns recent health events for a component.
func (r *ComponentsRepository) HealthHistory(componentID uuid.UUID, limit int) ([]domain.NocHealthEvent, error) {
	var events []domain.NocHealthEvent
	err := r.db.
		Where("component_id = ?", componentID).
		Order("occurred_at DESC").
		Limit(limit).
		Find(&events).Error
	return events, err
}

// DeleteOldHealthEvents purges events older than cutoff.
func (r *ComponentsRepository) DeleteOldHealthEvents(cutoff time.Time) error {
	return r.db.
		Where("occurred_at < ?", cutoff).
		Delete(&domain.NocHealthEvent{}).Error
}

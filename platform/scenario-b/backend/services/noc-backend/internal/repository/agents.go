// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/backend/services/noc-backend/internal/domain"
)

// AgentsRepository provides CRUD for NocAgent and NocProvisionedKey.
type AgentsRepository struct {
	db *gorm.DB
}

// NewAgentsRepository creates a new AgentsRepository.
func NewAgentsRepository(db *gorm.DB) *AgentsRepository {
	return &AgentsRepository{db: db}
}

// ProvisionKey stores a hashed API key bound to a spoke (idempotent — returns
// the existing record if the same raw key was already provisioned).
func (r *AgentsRepository) ProvisionKey(rawKey string, spokeID uuid.UUID, hint string) (*domain.NocProvisionedKey, error) {
	hash := sha256Hex(rawKey)
	pk := &domain.NocProvisionedKey{
		KeyHash: hash,
		SpokeID: spokeID,
		Hint:    hint,
	}
	if err := r.db.Where("key_hash = ?", hash).FirstOrCreate(pk).Error; err != nil {
		return nil, fmt.Errorf("agents: provision-key: %w", err)
	}
	return pk, nil
}

// FindProvisionedKey looks up a provisioned key by raw value.
func (r *AgentsRepository) FindProvisionedKey(rawKey string) (*domain.NocProvisionedKey, error) {
	hash := sha256Hex(rawKey)
	var pk domain.NocProvisionedKey
	if err := r.db.Where("key_hash = ?", hash).First(&pk).Error; err != nil {
		return nil, fmt.Errorf("agents: find-provisioned-key: %w", err)
	}
	return &pk, nil
}

// MarkKeyUsed sets UsedAt to now on first use.
func (r *AgentsRepository) MarkKeyUsed(keyID uuid.UUID) error {
	now := time.Now()
	return r.db.Model(&domain.NocProvisionedKey{}).
		Where("id = ? AND used_at IS NULL", keyID).
		Update("used_at", now).Error
}

// FindOrCreateAgent returns an existing agent by key hash, or creates one on first use.
func (r *AgentsRepository) FindOrCreateAgent(rawKey string, spokeID uuid.UUID, name string, pushInterval int) (*domain.NocAgent, error) {
	hash := sha256Hex(rawKey)
	var agent domain.NocAgent
	err := r.db.Where("api_key_hash = ?", hash).First(&agent).Error
	if err == nil {
		return &agent, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("agents: find-agent: %w", err)
	}

	agent = domain.NocAgent{
		SpokeID:             spokeID,
		Name:                name,
		ApiKeyHash:          hash,
		PushIntervalSeconds: pushInterval,
		Status:              "REACHABLE",
	}
	if err := r.db.Create(&agent).Error; err != nil {
		return nil, fmt.Errorf("agents: create-agent: %w", err)
	}
	return &agent, nil
}

// UpdateLastSeen refreshes the agent's LastSeenAt and Status.
func (r *AgentsRepository) UpdateLastSeen(agentID uuid.UUID) error {
	now := time.Now()
	return r.db.Model(&domain.NocAgent{}).
		Where("id = ?", agentID).
		Updates(map[string]any{"last_seen_at": now, "status": "REACHABLE"}).Error
}

// ListAgents returns all agents, optionally filtered by spoke.
func (r *AgentsRepository) ListAgents(spokeID *uuid.UUID) ([]domain.NocAgent, error) {
	q := r.db.Model(&domain.NocAgent{})
	if spokeID != nil {
		q = q.Where("spoke_id = ?", *spokeID)
	}
	var agents []domain.NocAgent
	if err := q.Find(&agents).Error; err != nil {
		return nil, fmt.Errorf("agents: list: %w", err)
	}
	return agents, nil
}

// MarkAgentsUnreachable sets status=UNREACHABLE for agents whose last_seen_at is stale.
// graceSeconds = push_interval_seconds * multiplier
func (r *AgentsRepository) MarkStaleAgentsUnreachable(multiplier int) error {
	return r.db.Exec(`
		UPDATE noc_agents
		SET status = 'UNREACHABLE'
		WHERE status = 'REACHABLE'
		  AND last_seen_at IS NOT NULL
		  AND last_seen_at < NOW() - (push_interval_seconds * ? * INTERVAL '1 second')`,
		multiplier,
	).Error
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)
}

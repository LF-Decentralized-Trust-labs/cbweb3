// SPDX-License-Identifier: Apache-2.0

// Package services provides the Circuit Breaker gate for Scenario B swap execution (FR-030 / H4).
package services

import (
	"context"
	"fmt"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"gorm.io/gorm"
)

// CircuitBreakerGateImpl implements the CircuitBreakerGate interface using the DB risk control state.
type CircuitBreakerGateImpl struct {
	db *gorm.DB
}

// NewCircuitBreakerGate creates a CircuitBreakerGateImpl.
func NewCircuitBreakerGate(db *gorm.DB) *CircuitBreakerGateImpl {
	return &CircuitBreakerGateImpl{db: db}
}

// IsHalted returns true if the circuit breaker is in HALTED state for the given pair (FR-030).
func (g *CircuitBreakerGateImpl) IsHalted(ctx context.Context, pair string) (bool, error) {
	var state domain.ScenarioBRiskControlState
	err := g.db.WithContext(ctx).
		Where("pool_pair = ?", pair).
		First(&state).Error

	if err == gorm.ErrRecordNotFound {
		// No record means the circuit breaker is not configured — allow by default
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("circuit breaker state query: %w", err)
	}
	return state.CircuitBreakerState == domain.CircuitBreakerHalted, nil
}

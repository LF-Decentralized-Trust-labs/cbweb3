// Package seeds provides initial data seeding for Scenario B endpoint contracts (FR-055 / data-model.md §2).
package seeds

import (
	"fmt"

	apidomain "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type endpointSeed struct {
	path   string
	method string
	domain apidomain.EndpointDomain
}

var v2Endpoints = []endpointSeed{
	// US1 — AMM
	{"/api/v2/amm/quote/exact-output", "GET", apidomain.EndpointDomainQuote},
	{"/api/v2/amm/swap/exact-output", "POST", apidomain.EndpointDomainSwap},
	{"/api/v2/amm/pool/:pair/status", "GET", apidomain.EndpointDomainPoolStatus},
	{"/api/v2/amm/liquidity/add", "POST", apidomain.EndpointDomainQuote},
	{"/api/v2/amm/liquidity/remove", "POST", apidomain.EndpointDomainQuote},
	// US2 — Bridge
	{"/api/v2/bridge/lock-mint", "POST", apidomain.EndpointDomainBridge},
	{"/api/v2/bridge/burn-unlock", "POST", apidomain.EndpointDomainBridge},
	{"/api/v2/bridge/positions", "GET", apidomain.EndpointDomainBridge},
	// US3 — Governance
	{"/api/v2/governance/circuit-breaker/pause", "POST", apidomain.EndpointDomainGovernanceRisk},
	{"/api/v2/governance/circuit-breaker/resume-request", "POST", apidomain.EndpointDomainGovernanceRisk},
	{"/api/v2/governance/circuit-breaker/resume-sign", "POST", apidomain.EndpointDomainGovernanceRisk},
	{"/api/v2/governance/circuit-breaker/status", "GET", apidomain.EndpointDomainGovernanceRisk},
	// US3 — Oversight
	{"/api/v2/oversight/disclosure-request", "POST", apidomain.EndpointDomainOversight},
	{"/api/v2/oversight/disclosure-sign", "POST", apidomain.EndpointDomainOversight},
	{"/api/v2/oversight/disclosure-status/:requestID", "GET", apidomain.EndpointDomainOversight},
}

// SeedEndpointContracts inserts the canonical v2 endpoint contracts via GORM Create.
// Idempotent: skips records that already exist (by path + method).
// No .sql files — GORM only (FR-055).
func SeedEndpointContracts(db *gorm.DB) error {
	for _, s := range v2Endpoints {
		seed := s
		var existing apidomain.ScenarioBEndpointContract
		result := db.Where("path = ? AND method = ?", seed.path, seed.method).First(&existing)
		if result.Error == nil {
			continue
		}

		contract := apidomain.ScenarioBEndpointContract{
			EndpointID: uuid.NewString(),
			Domain:     seed.domain,
			Path:       seed.path,
			Method:     seed.method,
			State:      apidomain.EndpointActive,
		}
		if err := db.Create(&contract).Error; err != nil {
			return fmt.Errorf("seed endpoint contract %s %s: %w", contract.Method, contract.Path, err)
		}
	}
	return nil
}

// Package init provides GORM-based schema initialisation for Scenario B.
package init

import (
	apidomain "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"gorm.io/gorm"
)

// RunAutoMigrate registers all Scenario B models and migrates the schema.
// No .sql migration files — GORM AutoMigrate only (FR-055).
func RunAutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		// Phase 2: Cutover artifacts (T033)
		&apidomain.ScenarioBApiCutoverPlan{},
		&apidomain.ScenarioBEndpointContract{},
		&apidomain.LegacyArtifactInventory{},
		&apidomain.InfrastructureReuseRegister{},

		// Phase 3 US1: risk control + pool monitoring (T032, T034)
		&apidomain.ScenarioBRiskControlState{},

		// Phase 4 US2: pool monitoring + liquidity positions (T057)
		&apidomain.PoolStateReading{},
		&apidomain.LiquidityAlert{},
		&apidomain.LiquidityPosition{},

		// Phase 5 US3: circuit breaker audit (T082)
		&apidomain.CircuitBreakerSignature{},

		// Phase 4 US2: bridge positions + relayer queue (FR-029 / FR-031)
		&apidomain.BridgedAssetPosition{},
		&apidomain.RelayerQueueItem{},

		// Phase 5 US3: oversight disclosure (FR-034 / FR-035 / FR-036)
		&apidomain.DisclosureRequest{},
		&apidomain.DisclosureSignature{},

		// 005-cooperative-liquidity: commit-reveal + fee distribution
		&apidomain.PoolCommit{},
		&apidomain.LPFeeEvent{},

		// 005-cooperative-liquidity Phase 8: multi-pair PairRegistry (D11 / T042)
		&apidomain.PairProposal{},

		// 009-commercial-cross-currency-swap: orchestrator tracking + quotes + rollback
		&apidomain.CrossCurrencySwapOperation{},
		&apidomain.SwapQuote{},
		&apidomain.SwapRollbackLog{},
		&apidomain.SwapRateLimitCounter{},

		// R1-10.1: configurable CB transfer limits + daily volume tracking
		&apidomain.TransferLimit{},
		&apidomain.TransferVolumeLog{},
	)
}

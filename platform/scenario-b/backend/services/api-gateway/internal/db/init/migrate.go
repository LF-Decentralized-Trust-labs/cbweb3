// SPDX-License-Identifier: Apache-2.0

// Package init provides GORM-based schema initialisation for Scenario B.
package init

import (
	"fmt"

	apidomain "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"gorm.io/gorm"
)

// legacySwapTxHashIndex is the pre-residue replay-protection index: unique on
// swap_tx_hash alone. Superseded by idx_bridge_swap_tx_leg, which is unique on
// (swap_tx_hash, leg) so a swap can carry both a settlement and a residue leg.
const (
	legacySwapTxHashIndex = "idx_bridge_swap_tx_hash"
	swapTxLegIndex        = "idx_bridge_swap_tx_leg"
)

// RunAutoMigrate registers all Scenario B models and migrates the schema.
// No .sql migration files — GORM AutoMigrate only (FR-055).
func RunAutoMigrate(db *gorm.DB) error {
	if err := autoMigrateModels(db); err != nil {
		return err
	}
	return dropLegacySwapTxHashIndex(db)
}

// dropLegacySwapTxHashIndex removes the single-column unique index once AutoMigrate has
// created its composite replacement.
//
// Order matters: the replacement is created first (by AutoMigrate) and the old index is
// dropped only after it is confirmed present, so replay protection is never absent — not
// even for the duration of the migration. If the replacement is missing, the old index
// stays and the error surfaces rather than leaving the table unprotected.
func dropLegacySwapTxHashIndex(db *gorm.DB) error {
	m := db.Migrator()
	pos := &apidomain.BridgedAssetPosition{}
	if !m.HasIndex(pos, legacySwapTxHashIndex) {
		return nil
	}
	if !m.HasIndex(pos, swapTxLegIndex) {
		return fmt.Errorf(
			"refusing to drop %s: replacement index %s was not created — bridge replay protection would be lost",
			legacySwapTxHashIndex, swapTxLegIndex,
		)
	}
	if err := m.DropIndex(pos, legacySwapTxHashIndex); err != nil {
		return fmt.Errorf("drop legacy index %s: %w", legacySwapTxHashIndex, err)
	}
	return nil
}

func autoMigrateModels(db *gorm.DB) error {
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

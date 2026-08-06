// SPDX-License-Identifier: Apache-2.0

// Package init provides GORM-based schema initialisation for Scenario B.
package init

import (
	"fmt"
	"log"

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
	if err := backfillBridgeDirection(db); err != nil {
		return err
	}
	return dropLegacySwapTxHashIndex(db)
}

// backfillBridgeDirection fills the direction of positions created before the column existed.
//
// Nothing on the position itself said which way it moved value — that is exactly the gap the
// column closes — so a legacy row is classified from three durable signals, ANY of which means it
// burns on the Hub:
//
//	a BURN_UNLOCK queue item      the relayer's own record of what it was asked to do
//	leg = RESIDUE                 a residue leg exists only to burn the unspent buffer back;
//	                              there is no other kind
//	burn_from_hub_address <> ''   written only by the cross-currency bridge-out handler, to name
//	                              the address the burn takes from
//
// The queue item alone is NOT enough, and the extra two are not redundancy. A burn position and
// its queue item are two statements: a row created in the window between them (the gap
// ensureBurnQueueItem closes going forward) has no queue item at all, and would otherwise fall
// through to IN — which would make the reconciliation count an outbound position as money the CB
// still holds, inflating its expectation and hiding a real shortfall behind it.
//
// Everything left really is inbound: that is what LockAndEnqueue produces, and it is the only
// producer whose positions can legitimately reach this point unclassified.
//
// Idempotent: it only touches rows whose direction is still empty or NULL — a column added to an
// existing table leaves one or the other depending on the default — so a re-run is a no-op and a
// direction written at creation is never overwritten.
func backfillBridgeDirection(db *gorm.DB) error {
	m := db.Migrator()
	pos := &apidomain.BridgedAssetPosition{}
	if !m.HasColumn(pos, "direction") {
		return nil
	}
	out := db.Model(pos).
		Where("(direction IS NULL OR direction = '')").
		Where(db.Where("EXISTS (SELECT 1 FROM relayer_queue_items q WHERE q.position_id = bridged_asset_positions.position_id AND q.event_type = ?)", "BURN_UNLOCK").
			Or("leg = ?", apidomain.BridgeLegResidue).
			Or("burn_from_hub_address <> ''")).
		Update("direction", apidomain.BridgeDirectionOut)
	if out.Error != nil {
		return fmt.Errorf("backfill bridge direction (OUT): %w", out.Error)
	}
	in := db.Model(pos).
		Where("(direction IS NULL OR direction = '')").
		Update("direction", apidomain.BridgeDirectionIn)
	if in.Error != nil {
		return fmt.Errorf("backfill bridge direction (IN): %w", in.Error)
	}
	if out.RowsAffected > 0 || in.RowsAffected > 0 {
		log.Printf("[migrate] backfilled bridge direction: %d OUT, %d IN", out.RowsAffected, in.RowsAffected)
	}
	return nil
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

		// Sovereign delegation of the Hub AMM swap (Step 2): the CB records each swap it
		// executed for a bank, keyed on the funding bridge-in position, so a retried
		// delegation never runs the swap twice.
		&apidomain.CrossCurrencyHubSwap{},

		// R1-10.1: configurable CB transfer limits + daily volume tracking
		&apidomain.TransferLimit{},
		&apidomain.TransferVolumeLog{},
	)
}

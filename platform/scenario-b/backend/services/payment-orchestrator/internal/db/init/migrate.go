// SPDX-License-Identifier: Apache-2.0

// Package init provides GORM-based schema initialisation for payment-orchestrator.
package init

import (
	podmain "github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"gorm.io/gorm"
)

// RunAutoMigrate ensures the schema the payment-orchestrator *writes* exists before the
// relayer worker starts.
//
// The api-gateway remains the authoritative owner of the shared bridged_asset_positions table
// (indexes and unique constraints are declared there). This call is a defensive, additive
// safety net for startup ordering: GORM AutoMigrate only adds missing columns/tables and never
// drops, so running it here is idempotent and converges with the gateway's migration whichever
// service starts first. Without it, if the orchestrator races ahead of the gateway, the burn
// confirmation write (hub_burn_tx_hash) fails after the on-chain burn already succeeded, and the
// retry re-burns (R2-H-12 double-burn trigger).
func RunAutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&podmain.BridgedAssetPosition{},
		&podmain.RelayerQueueItem{},
	)
}

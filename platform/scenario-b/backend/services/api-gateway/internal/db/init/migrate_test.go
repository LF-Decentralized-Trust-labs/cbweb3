// SPDX-License-Identifier: Apache-2.0

package init_test

import (
	"testing"

	"github.com/glebarez/sqlite" // pure-Go (no CGO) sqlite driver, test-only
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	dbinit "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/db/init"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

const (
	legacyIndex = "idx_bridge_swap_tx_hash"
	newIndex    = "idx_bridge_swap_tx_leg"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

// On a fresh schema only the composite index exists.
func TestRunAutoMigrate_CreatesCompositeSwapTxLegIndex(t *testing.T) {
	db := openTestDB(t)
	if err := dbinit.RunAutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	m := db.Migrator()
	pos := &domain.BridgedAssetPosition{}
	if !m.HasIndex(pos, newIndex) {
		t.Fatalf("expected %s to be created", newIndex)
	}
	if m.HasIndex(pos, legacyIndex) {
		t.Fatalf("expected %s not to exist on a fresh schema", legacyIndex)
	}
}

// Upgrading a table that still carries the single-column index: the replacement is created
// first and the legacy index is dropped only afterwards, so replay protection is continuous.
func TestRunAutoMigrate_DropsLegacyIndexAfterReplacement(t *testing.T) {
	db := openTestDB(t)

	// Simulate the pre-residue schema: table + legacy unique index, no leg column.
	if err := db.Exec(`CREATE TABLE bridged_asset_positions (
		position_id text PRIMARY KEY,
		owner_bank_id text NOT NULL,
		spoke_network text NOT NULL,
		native_asset text NOT NULL,
		mirrored_asset text NOT NULL,
		mirrored_amount text NOT NULL,
		bridge_state text NOT NULL DEFAULT 'LOCKING',
		relayer_retries integer NOT NULL DEFAULT 0,
		swap_tx_hash text DEFAULT '',
		correlation_id text DEFAULT ''
	)`).Error; err != nil {
		t.Fatalf("seed legacy table: %v", err)
	}
	if err := db.Exec(`CREATE UNIQUE INDEX ` + legacyIndex +
		` ON bridged_asset_positions(swap_tx_hash) WHERE swap_tx_hash <> ''`).Error; err != nil {
		t.Fatalf("seed legacy index: %v", err)
	}

	// A pre-existing settlement row must survive the migration and be backfilled to
	// SETTLEMENT, which is what it is.
	if err := db.Exec(`INSERT INTO bridged_asset_positions
		(position_id, owner_bank_id, spoke_network, native_asset, mirrored_asset, mirrored_amount, swap_tx_hash)
		VALUES ('legacy-1','bank-b','spoke-ars','tCeBM-ARS','W-ARS','100','0xlegacy')`).Error; err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}

	if err := dbinit.RunAutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	m := db.Migrator()
	pos := &domain.BridgedAssetPosition{}
	if !m.HasIndex(pos, newIndex) {
		t.Fatalf("expected %s to be created", newIndex)
	}
	if m.HasIndex(pos, legacyIndex) {
		t.Fatalf("expected %s to be dropped: while it exists, a residue leg collides with its settlement", legacyIndex)
	}

	var row domain.BridgedAssetPosition
	if err := db.Where("position_id = ?", "legacy-1").First(&row).Error; err != nil {
		t.Fatalf("legacy row must survive the migration: %v", err)
	}
	if row.Leg != domain.BridgeLegSettlement {
		t.Fatalf("legacy row should be backfilled to SETTLEMENT, got %q", row.Leg)
	}
	if row.MirroredAmount != "100" {
		t.Fatalf("legacy amount must be untouched, got %q", row.MirroredAmount)
	}
}

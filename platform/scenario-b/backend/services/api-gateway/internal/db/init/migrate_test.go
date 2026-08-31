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

// --- direction backfill ---
//
// The Hub reconciliation asks what a central bank HOLDS, which requires knowing which way each
// position moved value. Nothing on the row said so before the direction column, so pre-existing
// rows are classified from the relayer queue. Getting this wrong is silent: a bridge-out
// misclassified as a bridge-in inflates the CB's expectation and hides a real shortfall behind it.

// seedLegacyPosition inserts a row the way a pre-column deployment left it. direction is passed as
// raw SQL because both shapes occur in the field: a column added WITH a default leaves ”, added
// without one leaves NULL, and the backfill has to catch either.
func seedLegacyPosition(t *testing.T, db *gorm.DB, id, leg, direction, burnFrom string) {
	t.Helper()
	if err := db.Exec(`INSERT INTO bridged_asset_positions
		(position_id, owner_bank_id, spoke_network, native_asset, mirrored_asset, mirrored_amount,
		 bridge_state, leg, direction, burn_from_hub_address)
		VALUES (?,'bank-a','spoke-brl','tCeBM-BRL','W-BRL','100','ACTIVE',?,`+direction+`,?)`,
		id, leg, burnFrom).Error; err != nil {
		t.Fatalf("seed position %s: %v", id, err)
	}
}

// seedDirectionlessPosition is the common case: a settlement leg with an empty direction.
func seedDirectionlessPosition(t *testing.T, db *gorm.DB, id, amount string) {
	t.Helper()
	seedLegacyPosition(t, db, id, "SETTLEMENT", "''", "")
}

func seedQueueItem(t *testing.T, db *gorm.DB, positionID, eventType string) {
	t.Helper()
	item := &domain.RelayerQueueItem{
		ItemID: "item-" + positionID + "-" + eventType, IdempotencyKey: eventType + ":" + positionID,
		EventType: eventType, PositionID: positionID, State: domain.RelayerStateCompleted,
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("seed queue item for %s: %v", positionID, err)
	}
}

func directionOf(t *testing.T, db *gorm.DB, id string) domain.BridgeDirection {
	t.Helper()
	var row domain.BridgedAssetPosition
	if err := db.Where("position_id = ?", id).First(&row).Error; err != nil {
		t.Fatalf("read %s: %v", id, err)
	}
	return row.Direction
}

// The queue's event type is the only durable record of direction for a legacy row.
func TestRunAutoMigrate_BackfillsDirectionFromTheRelayerQueue(t *testing.T) {
	db := openTestDB(t)
	if err := dbinit.RunAutoMigrate(db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}

	seedDirectionlessPosition(t, db, "p-burn", "100")
	seedQueueItem(t, db, "p-burn", "BURN_UNLOCK")
	seedDirectionlessPosition(t, db, "p-mint", "200")
	seedQueueItem(t, db, "p-mint", "LOCK_MINT")
	// A settlement with no queue item at all: that is what LockAndEnqueue leaves when its own
	// enqueue failed, and it only produces inbound, so IN is the correct fallback.
	seedDirectionlessPosition(t, db, "p-orphan", "300")
	// A column added without a default leaves NULL rather than '' — the likelier field shape.
	seedLegacyPosition(t, db, "p-null", "SETTLEMENT", "NULL", "")

	if err := dbinit.RunAutoMigrate(db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	for id, want := range map[string]domain.BridgeDirection{
		"p-burn":   domain.BridgeDirectionOut,
		"p-mint":   domain.BridgeDirectionIn,
		"p-orphan": domain.BridgeDirectionIn,
		"p-null":   domain.BridgeDirectionIn,
	} {
		if got := directionOf(t, db, id); got != want {
			t.Fatalf("%s backfilled to %q, want %q", id, got, want)
		}
	}
}

// The queue item is not the only signal, and the other two are not redundancy. A burn position and
// its queue item are two statements, so a legacy row created in the window between them has no
// queue item — and falling through to IN would make the reconciliation count an OUTBOUND position
// as money the CB still holds, inflating its expectation and masking a real shortfall.
func TestRunAutoMigrate_BackfillsOutboundWithNoQueueItem(t *testing.T) {
	db := openTestDB(t)
	if err := dbinit.RunAutoMigrate(db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}

	// A residue leg exists only to burn the unspent buffer back; there is no inbound residue.
	seedLegacyPosition(t, db, "p-residue", "RESIDUE", "''", "")
	// burn_from_hub_address is written only by the cross-currency bridge-out handler, and names
	// the address the burn takes from — so its presence is itself the direction.
	seedLegacyPosition(t, db, "p-burnfrom", "SETTLEMENT", "''", "0xCBHUB")
	// Same shape, but NULL instead of '', since a column added without a default leaves NULL.
	seedLegacyPosition(t, db, "p-residue-null", "RESIDUE", "NULL", "")

	if err := dbinit.RunAutoMigrate(db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	for _, id := range []string{"p-residue", "p-burnfrom", "p-residue-null"} {
		if got := directionOf(t, db, id); got != domain.BridgeDirectionOut {
			t.Fatalf("%s backfilled to %q, want OUT — an outbound position with no queue item was read as held", id, got)
		}
	}
}

// A direction written at creation is authoritative: the queue can carry both a LOCK_MINT and a
// later BURN_UNLOCK for the same position, so re-deriving it would overwrite the truth with a
// guess. The backfill must only touch rows that have none.
func TestRunAutoMigrate_BackfillDoesNotOverwriteAnExistingDirection(t *testing.T) {
	db := openTestDB(t)
	if err := dbinit.RunAutoMigrate(db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}

	pos := &domain.BridgedAssetPosition{
		PositionID: "p-known", OwnerBankID: "bank-a", SpokeNetwork: "spoke-brl",
		NativeAsset: "tCeBM-BRL", MirroredAsset: "W-BRL", MirroredAmount: "500",
		BridgeState: domain.BridgeStateActive, Leg: domain.BridgeLegSettlement,
		Direction: domain.BridgeDirectionOut,
	}
	if err := db.Create(pos).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	// A queue item that would classify it the other way if the backfill re-derived it.
	seedQueueItem(t, db, "p-known", "LOCK_MINT")

	if err := dbinit.RunAutoMigrate(db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if got := directionOf(t, db, "p-known"); got != domain.BridgeDirectionOut {
		t.Fatalf("direction was overwritten to %q — a value set at creation must win", got)
	}
}

// Migrations run on every boot, so a second pass must change nothing.
func TestRunAutoMigrate_BackfillIsIdempotent(t *testing.T) {
	db := openTestDB(t)
	if err := dbinit.RunAutoMigrate(db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	seedDirectionlessPosition(t, db, "p-burn", "100")
	seedQueueItem(t, db, "p-burn", "BURN_UNLOCK")

	if err := dbinit.RunAutoMigrate(db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	first := directionOf(t, db, "p-burn")
	if err := dbinit.RunAutoMigrate(db); err != nil {
		t.Fatalf("third migrate: %v", err)
	}
	if second := directionOf(t, db, "p-burn"); second != first {
		t.Fatalf("a re-run changed the direction from %q to %q", first, second)
	}
	if first != domain.BridgeDirectionOut {
		t.Fatalf("direction = %q, want OUT", first)
	}
}

// SPDX-License-Identifier: Apache-2.0

package init_test

import (
	"strings"
	"testing"

	dbinit "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/db/init"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/glebarez/sqlite" // pure-Go (no CGO) sqlite driver, test-only
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

// A table that already carries duplicates cannot take the unique index, and the driver-level
// failure names a constraint and nothing else. Each duplicate is a burn and a mint that ran
// twice for one notification, so the migration stops with something an operator can act on.
func TestRunAutoMigrate_RefusesATableWithDuplicateBridgeInCorrelations(t *testing.T) {
	db := newDB(t)

	// Build the pre-fix schema: same model, without the unique index the fix adds.
	type legacyPosition struct {
		PositionID    string `gorm:"primaryKey;column:position_id"`
		OwnerBankID   string `gorm:"column:owner_bank_id"`
		SpokeNetwork  string `gorm:"column:spoke_network"`
		NativeAsset   string `gorm:"column:native_asset"`
		MirroredAsset string `gorm:"column:mirrored_asset"`
		Direction     string `gorm:"column:direction"`
		CorrelationID string `gorm:"column:correlation_id"`
		BridgeState   string `gorm:"column:bridge_state"`
	}
	if err := db.Table("bridged_asset_positions").AutoMigrate(&legacyPosition{}); err != nil {
		t.Fatalf("legacy migrate: %v", err)
	}
	for _, id := range []string{"pos-1", "pos-2"} {
		if err := db.Table("bridged_asset_positions").Create(&legacyPosition{
			PositionID: id, OwnerBankID: "bank-a", SpokeNetwork: "spoke-brl",
			NativeAsset: "tCeBM-BRL", MirroredAsset: "W-BRL",
			Direction: string(domain.BridgeDirectionIn), CorrelationID: "corr-double-charged",
			BridgeState: string(domain.BridgeStateActive),
		}).Error; err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}

	err := dbinit.RunAutoMigrate(db)
	if err == nil {
		t.Fatal("migration accepted a table with duplicate bridge-in correlations")
	}
	if !strings.Contains(err.Error(), "corr-double-charged") {
		t.Fatalf("error must name the affected correlation so it can be reconciled, got: %v", err)
	}
}

// The guard must not fire on rows that legitimately share a correlation: the outbound legs of
// the same swap, and rows with no correlation at all.
func TestRunAutoMigrate_AcceptsSharedCorrelationsOnOtherDirections(t *testing.T) {
	db := newDB(t)
	if err := dbinit.RunAutoMigrate(db); err != nil {
		t.Fatalf("first migrate: %v", err)
	}

	rows := []domain.BridgedAssetPosition{
		{PositionID: "in-1", OwnerBankID: "bank-a", SpokeNetwork: "spoke-brl", NativeAsset: "tCeBM-BRL",
			MirroredAsset: "W-BRL", Direction: domain.BridgeDirectionIn, CorrelationID: "corr-1"},
		{PositionID: "out-1", OwnerBankID: "bank-b", SpokeNetwork: "spoke-ars", NativeAsset: "tCeBM-ARS",
			MirroredAsset: "W-ARS", Direction: domain.BridgeDirectionOut, CorrelationID: "corr-1"},
		{PositionID: "out-2", OwnerBankID: "bank-a", SpokeNetwork: "spoke-brl", NativeAsset: "tCeBM-BRL",
			MirroredAsset: "W-BRL", Direction: domain.BridgeDirectionOut, CorrelationID: "corr-1",
			Leg: domain.BridgeLegResidue},
		{PositionID: "plain-1", OwnerBankID: "bank-a", SpokeNetwork: "spoke-brl", NativeAsset: "tCeBM-BRL",
			MirroredAsset: "W-BRL", Direction: domain.BridgeDirectionIn},
		{PositionID: "plain-2", OwnerBankID: "bank-a", SpokeNetwork: "spoke-brl", NativeAsset: "tCeBM-BRL",
			MirroredAsset: "W-BRL", Direction: domain.BridgeDirectionIn},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("insert %s: %v", rows[i].PositionID, err)
		}
	}

	// Re-running the migration over that data must stay clean — it is the ordinary case.
	if err := dbinit.RunAutoMigrate(db); err != nil {
		t.Fatalf("re-migrate over legitimate shared correlations: %v", err)
	}
}

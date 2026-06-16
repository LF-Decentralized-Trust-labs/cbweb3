// SPDX-License-Identifier: Apache-2.0

package services

import (
	"testing"

	"github.com/glebarez/sqlite" // pure-Go (no CGO) sqlite driver, test-only
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// newTestDB returns an in-memory sqlite *gorm.DB with the given models migrated.
// Pure-Go driver keeps CI hermetic (no CGO, no external Postgres).
func newTestDB(t *testing.T, models ...interface{}) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if len(models) > 0 {
		if err := db.AutoMigrate(models...); err != nil {
			t.Fatalf("migrate: %v", err)
		}
	}
	return db
}

func newBridgeDB(t *testing.T) *gorm.DB {
	return newTestDB(t, &domain.BridgedAssetPosition{}, &domain.RelayerQueueItem{})
}

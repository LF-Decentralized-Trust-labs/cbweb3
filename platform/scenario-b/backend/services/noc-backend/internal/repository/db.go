// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/backend/services/noc-backend/internal/domain"
)

// Connect opens a PostgreSQL connection and runs AutoMigrate for all NOC models.
func Connect(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("repository: connecting to database: %w", err)
	}

	if err := db.AutoMigrate(
		&domain.NocSpoke{},
		&domain.NocProvisionedKey{},
		&domain.NocAgent{},
		&domain.NocComponent{},
		&domain.NocHealthEvent{},
		&domain.NocAlert{},
		&domain.NocIncident{},
		&domain.NocSloMetric{},
		&domain.NocTransactionEvent{},
		&domain.NocContainerLog{},
		&domain.NocLogSnapshot{},
		&domain.NocAuditEntry{},
	); err != nil {
		return nil, fmt.Errorf("repository: auto-migrate: %w", err)
	}

	return db, nil
}

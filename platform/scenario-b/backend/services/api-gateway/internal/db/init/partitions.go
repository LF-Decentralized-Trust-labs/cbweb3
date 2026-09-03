// SPDX-License-Identifier: Apache-2.0

// Package init provides declarative time-based partitioning DDL for high-cardinality tables.
// Partitions are created via db.Exec() using Postgres PARTITION BY RANGE (FR-046 / Decision 14).
package init

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// CreatePartitions creates monthly time-based partitions for high-cardinality Scenario B tables.
// The function is idempotent — partitions are created with IF NOT EXISTS.
func CreatePartitions(db *gorm.DB) error {
	now := time.Now()

	// Create partitions for the current month and the next two months.
	for i := 0; i < 3; i++ {
		month := now.AddDate(0, i, 0)
		if err := createMonthlyPartition(db, "swap_order_scenario_b", month); err != nil {
			return fmt.Errorf("partition swap_order_scenario_b: %w", err)
		}
		if err := createMonthlyPartition(db, "pool_state_readings", month); err != nil {
			return fmt.Errorf("partition pool_state_readings: %w", err)
		}
	}
	return nil
}

// createMonthlyPartition creates a PARTITION BY RANGE child table for the given month.
func createMonthlyPartition(db *gorm.DB, parentTable string, month time.Time) error {
	year := month.Year()
	m := int(month.Month())
	next := month.AddDate(0, 1, 0)

	partitionName := fmt.Sprintf("%s_%04d_%02d", parentTable, year, m)
	startDate := fmt.Sprintf("%04d-%02d-01", year, m)
	endDate := fmt.Sprintf("%04d-%02d-01", next.Year(), int(next.Month()))

	ddl := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s
PARTITION OF %s
FOR VALUES FROM ('%s') TO ('%s');`,
		partitionName, parentTable, startDate, endDate)

	return db.Exec(ddl).Error
}

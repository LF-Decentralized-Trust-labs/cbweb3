// SPDX-License-Identifier: Apache-2.0

// Package init provides GORM-based schema initialisation for payment-orchestrator.
package init

import "gorm.io/gorm"

// RunAutoMigrate migrates the payment-orchestrator Scenario B schema.
func RunAutoMigrate(db *gorm.DB) error {
	// TODO: add model types here as they are implemented
	return nil
}

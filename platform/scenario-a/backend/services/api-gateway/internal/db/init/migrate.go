// SPDX-License-Identifier: Apache-2.0

// Package init provides GORM-based schema initialisation for Scenario A api-gateway.
package init

import (
	apidomain "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"gorm.io/gorm"
)

// RunAutoMigrate runs GORM AutoMigrate for all api-gateway-owned tables.
func RunAutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		// Investigation Module: disclosure requests + co-signatures (FR-034 / FR-035 / FR-036)
		&apidomain.DisclosureRequest{},
		&apidomain.DisclosureSignature{},
		// Central Bank: settled inter-bank PvP legs reported by settling orchestrators,
		// serving the credit side of commercial-bank statements.
		&apidomain.PvPSettledLeg{},
	)
}

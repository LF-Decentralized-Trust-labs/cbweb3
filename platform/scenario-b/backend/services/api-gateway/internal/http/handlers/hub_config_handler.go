// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"os"

	"github.com/gofiber/fiber/v2"
)

// GetHubContractsConfig exposes the hub AMM and registry contract addresses so that
// frontends and peer gateways can discover the on-chain source of truth for pairs.
// Addresses are read from the AMM_CONTRACT_ADDRESS, PAIR_REGISTRY_CONTRACT_ADDRESS and
// CURRENCY_REGISTRY_CONTRACT_ADDRESS env vars. Unset values are returned as empty strings.
// Always returns 200.
func GetHubContractsConfig(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"amm_address":       os.Getenv("AMM_CONTRACT_ADDRESS"),
		"pair_registry":     os.Getenv("PAIR_REGISTRY_CONTRACT_ADDRESS"),
		"currency_registry": os.Getenv("CURRENCY_REGISTRY_CONTRACT_ADDRESS"),
	})
}

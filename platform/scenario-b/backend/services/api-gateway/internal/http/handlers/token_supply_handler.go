// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
)

// SovereignSupply is this Central Bank's own wrapped token (W-tCeBM_<CUR>) outstanding
// on the hub — the amount of its domestic reserves currently bridged in.
//
// It is deliberately scoped to ONE currency. tCeBM is deployed per currency and per
// layer (domestic on each spoke, wrapped on the hub), so a network-wide "total tCeBM
// supply" would mix currencies and double-count the same backing: the domestic token
// locked in the SpokeBridge and the wrapped token minted against it are the same money.
type SovereignSupply struct {
	Symbol       string `json:"symbol"`
	TokenAddress string `json:"token_address"`
	TotalSupply  string `json:"total_supply"` // base units (wei), decimal string
	Decimals     uint8  `json:"decimals"`
}

// SovereignSupplyReader reads the wrapped-token supply for this gateway's own currency.
type SovereignSupplyReader interface {
	SovereignSupply(ctx context.Context) (*SovereignSupply, error)
}

// TokenSupplyHandler serves GET /api/v2/hub/token/supply.
type TokenSupplyHandler struct {
	reader SovereignSupplyReader
}

// NewTokenSupplyHandler creates a TokenSupplyHandler backed by the given reader.
func NewTokenSupplyHandler(reader SovereignSupplyReader) *TokenSupplyHandler {
	return &TokenSupplyHandler{reader: reader}
}

// GetSovereignSupply handles GET /api/v2/hub/token/supply.
//
// Read-only aggregate of public on-chain state, so it is unauthenticated — the same
// policy as GET /api/v2/hub/currencies. A hub RPC failure is a 502, never a zero:
// a fabricated zero would read as "nothing is bridged" to a supervisor.
//
// The cause is logged and not returned. The reader wraps the hub RPC URL and the token
// address into its errors, and this endpoint takes no credential — echoing them would
// hand the hub's internal address to any caller. Mirrors GET /api/v2/hub/currencies.
func (h *TokenSupplyHandler) GetSovereignSupply(c *fiber.Ctx) error {
	supply, err := h.reader.SovereignSupply(c.Context())
	if err != nil {
		log.Printf("[token-supply] read sovereign token supply: %v", err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "failed to read sovereign token supply",
			"code":  "ONCHAIN_ERROR",
		})
	}
	return c.Status(fiber.StatusOK).JSON(supply)
}

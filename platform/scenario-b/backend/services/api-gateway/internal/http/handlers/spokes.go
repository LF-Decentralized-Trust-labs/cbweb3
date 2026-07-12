// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"

	"github.com/gofiber/fiber/v2"
)

// SpokeRegistrar registers a spoke's central bank as a participant on the hub
// IdentityRegistry (via the compliance service, whose signer holds
// GOVERNANCE_ROLE). Implemented by the compliance gRPC adapter.
type SpokeRegistrar interface {
	RegisterParticipantOnChain(ctx context.Context, walletAddress, institutionName, role, bankCode string) (txHash string, alreadyRegistered bool, err error)
	// RegisterCurrencyOnChain asks the hub to deploy a founding central bank's
	// bridge token (W-token) and register its sovereign currency on-chain.
	RegisterCurrencyOnChain(ctx context.Context, currency, cbAddress, spokeID string) (symbol, tokenAddr string, alreadyRegistered bool, txHash string, err error)
	// RegisterPairOnChain asks the hub to deploy the sovereign-pair AMM over two
	// registered W-tokens and register the pair (proposePair + confirmPair).
	RegisterPairOnChain(ctx context.Context, currencyA, currencyB, pairID string) (ammAddr string, alreadyRegistered bool, txHash string, err error)
}

// SpokesHandler serves the internal machine-to-machine spoke self-registration
// endpoint used by the toolkit at found-spoke time (guarded by X-Relay-Auth).
type SpokesHandler struct {
	registrar SpokeRegistrar
}

// NewSpokesHandler constructs a SpokesHandler.
func NewSpokesHandler(r SpokeRegistrar) *SpokesHandler {
	return &SpokesHandler{registrar: r}
}

// RegisterSpoke handles POST /internal/v1/spokes/register. A founding central
// bank self-registers its spoke on the hub: the hub compliance service performs
// the on-chain registerParticipant with role ROLE_CENTRAL_BANK. Idempotent.
func (h *SpokesHandler) RegisterSpoke(c *fiber.Ctx) error {
	var body struct {
		SpokeID         string `json:"spoke_id"`
		CBAddress       string `json:"cb_address"`
		InstitutionName string `json:"institution_name"`
		Role            string `json:"role"`
		BankCode        string `json:"bank_code"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if body.CBAddress == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cb_address is required"})
	}
	role := body.Role
	if role == "" {
		role = "ROLE_CENTRAL_BANK"
	}
	txHash, already, err := h.registrar.RegisterParticipantOnChain(
		c.UserContext(), body.CBAddress, body.InstitutionName, role, body.BankCode)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"spoke_id":           body.SpokeID,
		"cb_address":         body.CBAddress,
		"role":               role,
		"already_registered": already,
		"tx_hash":            txHash,
	})
}

// RegisterSpokeCurrency handles POST /internal/v1/spokes/register-currency. A
// founding central bank asks the hub to deploy its bridge token (W-token) and
// register its sovereign currency on-chain. The hub compliance service performs
// the deploy + on-chain registration. Idempotent.
func (h *SpokesHandler) RegisterSpokeCurrency(c *fiber.Ctx) error {
	var body struct {
		Currency  string `json:"currency"`
		CBAddress string `json:"cb_address"`
		SpokeID   string `json:"spoke_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if body.CBAddress == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "cb_address is required"})
	}
	if body.Currency == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "currency is required"})
	}
	symbol, tokenAddr, already, txHash, err := h.registrar.RegisterCurrencyOnChain(
		c.UserContext(), body.Currency, body.CBAddress, body.SpokeID)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"symbol":             symbol,
		"token_address":      tokenAddr,
		"already_registered": already,
		"tx_hash":            txHash,
	})
}

// RegisterSpokePair handles POST /internal/v1/spokes/register-pair. The hub
// deploys the sovereign-pair AMM over two registered W-tokens and registers the
// pair (proposePair + confirmPair) — the corridor both CBs agreed to open.
// Idempotent.
func (h *SpokesHandler) RegisterSpokePair(c *fiber.Ctx) error {
	var body struct {
		CurrencyA string `json:"currency_a"`
		CurrencyB string `json:"currency_b"`
		PairID    string `json:"pair_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if body.CurrencyA == "" || body.CurrencyB == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "currency_a and currency_b are required"})
	}
	ammAddr, already, txHash, err := h.registrar.RegisterPairOnChain(
		c.UserContext(), body.CurrencyA, body.CurrencyB, body.PairID)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"pair_id":            body.PairID,
		"amm_address":        ammAddr,
		"already_registered": already,
		"tx_hash":            txHash,
	})
}

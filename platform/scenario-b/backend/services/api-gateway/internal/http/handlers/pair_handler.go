// SPDX-License-Identifier: Apache-2.0

// Package handlers provides HTTP handlers for PairRegistry operations (D9 — 005-cooperative-liquidity).
// Exposes three endpoints:
//
//	POST /api/v2/amm/pairs/propose  — Central Bank of tokenA proposes a new pair.
//	POST /api/v2/amm/pairs/confirm  — Central Bank of tokenB confirms a proposed pair.
//	GET  /api/v2/amm/pairs          — List all active pairs (served from DB, no RPC call).
package handlers

import (
	"errors"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// PairHandler handles PairRegistry HTTP operations.
type PairHandler struct {
	svc services.PairServiceIface
}

// NewPairHandler creates a PairHandler backed by the given service.
func NewPairHandler(svc services.PairServiceIface) *PairHandler {
	return &PairHandler{svc: svc}
}

// ProposePair handles POST /api/v2/amm/pairs/propose (FR-017 / SC-009).
func (h *PairHandler) ProposePair(c *fiber.Ctx) error {
	var req services.PairProposeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.PairID == "" || req.TokenAAddress == "" || req.TokenBAddress == "" ||
		req.AMMAddress == "" || req.ProposerCB == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "pair_id, token_a_address, token_b_address, amm_address, proposer_cb are required",
		})
	}

	result, err := h.svc.ProposePair(c.Context(), req)
	if err != nil {
		return pairErrorResponse(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

// ConfirmPair handles POST /api/v2/amm/pairs/confirm (FR-017 / SC-009).
func (h *PairHandler) ConfirmPair(c *fiber.Ctx) error {
	var req services.PairConfirmRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.PairID == "" || req.ConfirmerCB == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "pair_id, confirmer_cb are required",
		})
	}

	result, err := h.svc.ConfirmPair(c.Context(), req)
	if err != nil {
		return pairErrorResponse(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(result)
}

// ListPairs handles GET /api/v2/amm/pairs (FR-017).
// Served from the DB pair_proposals table; no on-chain RPC call required (D11).
func (h *PairHandler) ListPairs(c *fiber.Ctx) error {
	pairs, err := h.svc.ListActivePairs(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list pairs"})
	}
	resp := make([]fiber.Map, len(pairs))
	for i, p := range pairs {
		resp[i] = fiber.Map{
			"pair_id":         p.PairID,
			"status":          p.Status,
			"token_a_address": p.TokenAAddress,
			"token_b_address": p.TokenBAddress,
			"amm_address":     p.AMMAddress,
			"proposer_cb":     p.ProposerCB,
			"confirmer_cb":    p.ConfirmerCB,
			"proposed_at":     p.ProposedAt,
			"confirmed_at":    p.ConfirmedAt,
		}
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"pairs": resp})
}

// pairErrorResponse maps domain errors to HTTP status codes.
func pairErrorResponse(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, services.ErrPairAlreadyExists):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": err.Error(), "code": "PAIR_ALREADY_EXISTS",
		})
	case errors.Is(err, services.ErrPairNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(), "code": "PAIR_NOT_FOUND",
		})
	case errors.Is(err, services.ErrPairAlreadyActive):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": err.Error(), "code": "PAIR_ALREADY_ACTIVE",
		})
	case errors.Is(err, services.ErrNotCentralBankOfTokenA):
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": err.Error(), "code": "NOT_CENTRAL_BANK_OF_TOKEN_A",
		})
	case errors.Is(err, services.ErrNotCentralBankOfTokenB):
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": err.Error(), "code": "NOT_CENTRAL_BANK_OF_TOKEN_B",
		})
	default:
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
}

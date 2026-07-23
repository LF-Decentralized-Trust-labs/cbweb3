// SPDX-License-Identifier: Apache-2.0

// Package handlers provides the AMM swap HTTP handler for Scenario B (FR-027 / FR-057 / FR-058 / FR-059).
package handlers

import (
	"context"
	"errors"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

// SwapServiceIface is the interface consumed by SwapHandler.
type SwapServiceIface interface {
	Execute(ctx context.Context, req services.SwapRequest) (*services.SwapResult, error)
}

// SwapHandler handles POST /api/v2/amm/swap/exact-output.
type SwapHandler struct {
	svc SwapServiceIface
}

// NewSwapHandler creates a SwapHandler.
func NewSwapHandler(svc SwapServiceIface) *SwapHandler {
	return &SwapHandler{svc: svc}
}

// swapExactOutputRequest is the JSON body for a swap request.
type swapExactOutputRequest struct {
	Pair                 string `json:"pair"`
	AmountOut            string `json:"amount_out"`
	MaxAmountIn          string `json:"max_amount_in"`
	PayerID              string `json:"payer_id"`
	BeneficiaryID        string `json:"beneficiary_id"`
	ZKPointerPayer       string `json:"zk_pointer_payer"`
	ZKPointerBeneficiary string `json:"zk_pointer_beneficiary"`
}

// SwapExactOutput executes an exact-output swap.
// Requires commercial_bank role (FR-056).
// Performance gate: p95 <= 6s (SC-022).
func (h *SwapHandler) SwapExactOutput(c *fiber.Ctx) error {
	var req swapExactOutputRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":      "invalid request body",
			"error_code": "INVALID_REQUEST",
		})
	}

	if req.Pair == "" || req.AmountOut == "" || req.MaxAmountIn == "" ||
		req.PayerID == "" || req.BeneficiaryID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":      "pair, amount_out, max_amount_in, payer_id, beneficiary_id are required",
			"error_code": "INVALID_REQUEST",
		})
	}

	// R2-H-9 / R2-H-10: the payer is the party being debited, so its identity must
	// be the authenticated caller — never trusted from the request body. Reject a
	// swap whose payer_id diverges from the JWT BankID, mirroring RemoveLiquidity.
	if claims, ok := c.Locals("claims").(domain.TokenClaims); ok && claims.BankID != "" {
		if req.PayerID != claims.BankID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error":      "payer_id mismatch — must match authenticated identity",
				"error_code": "PAYER_ID_MISMATCH",
			})
		}
	}

	result, err := h.svc.Execute(c.Context(), services.SwapRequest{
		Pair:                 req.Pair,
		AmountOut:            req.AmountOut,
		MaxAmountIn:          req.MaxAmountIn,
		PayerID:              req.PayerID,
		BeneficiaryID:        req.BeneficiaryID,
		ZKPointerPayer:       req.ZKPointerPayer,
		ZKPointerBeneficiary: req.ZKPointerBeneficiary,
	})
	if err != nil {
		return h.handleSwapError(c, err)
	}

	return c.JSON(result)
}

func (h *SwapHandler) handleSwapError(c *fiber.Ctx, err error) error {
	var execErr *domain.SwapExecError
	if errors.As(err, &execErr) {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":      string(execErr.Code),
			"error_code": string(execErr.Code),
		})
	}

	var zkErr *services.ZKValidationError
	if errors.As(err, &zkErr) {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":      zkErr.Error(),
			"error_code": string(domain.ErrCodeZKValidationFailed),
		})
	}

	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		"error": "swap execution failed",
	})
}

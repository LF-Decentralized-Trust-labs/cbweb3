// Package handlers provides internal HTTP endpoints for CB transfer limit enforcement (R1-10.1).
// These endpoints are called by commercial bank gateways via RemoteTransferLimitChecker and are
// protected by X-Relay-Auth. They must never be exposed publicly.
package handlers

import (
	"errors"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

// TransferLimitInternalHandler exposes check-and-deduct / restore to commercial bank gateways.
// Registered only on CB api-gateways (nil on commercial banks).
type TransferLimitInternalHandler struct {
	checker services.TransferLimitCheckerIface
}

// NewTransferLimitInternalHandler creates a TransferLimitInternalHandler.
func NewTransferLimitInternalHandler(checker services.TransferLimitCheckerIface) *TransferLimitInternalHandler {
	return &TransferLimitInternalHandler{checker: checker}
}

type internalCheckRequest struct {
	PayerBankID string `json:"payer_bank_id"`
	Currency    string `json:"currency"`
	AmountHuman string `json:"amount_human"`
}

type internalRestoreRequest struct {
	PayerBankID string `json:"payer_bank_id"`
	Currency    string `json:"currency"`
	AmountHuman string `json:"amount_human"`
}

// HandleCheckAndDeduct handles POST /internal/v2/transfer-limits/check-and-deduct.
// Returns 200 OK, 422 TRANSFER_LIMIT_EXCEEDED, or 500 on unexpected error.
func (h *TransferLimitInternalHandler) HandleCheckAndDeduct(c *fiber.Ctx) error {
	var req internalCheckRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.PayerBankID == "" || req.AmountHuman == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "payer_bank_id and amount_human required"})
	}

	if err := h.checker.CheckAndDeduct(c.Context(), req.PayerBankID, req.Currency, req.AmountHuman); err != nil {
		var limitErr *services.ErrTransferLimitExceeded
		if errors.As(err, &limitErr) {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
				"error":        err.Error(),
				"error_code":   "TRANSFER_LIMIT_EXCEEDED",
				"payer_bank_id": limitErr.PayerBankID,
				"currency":     limitErr.Currency,
				"max_amount":   limitErr.MaxAmount,
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

// HandleRestore handles POST /internal/v2/transfer-limits/restore.
// Always returns 200 — Restore is best-effort and errors are swallowed.
func (h *TransferLimitInternalHandler) HandleRestore(c *fiber.Ctx) error {
	var req internalRestoreRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.PayerBankID == "" || req.AmountHuman == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "payer_bank_id and amount_human required"})
	}

	h.checker.Restore(c.Context(), req.PayerBankID, req.Currency, req.AmountHuman)
	return c.JSON(fiber.Map{"ok": true})
}

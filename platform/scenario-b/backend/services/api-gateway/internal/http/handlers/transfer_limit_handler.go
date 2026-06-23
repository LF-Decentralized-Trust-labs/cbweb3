// SPDX-License-Identifier: Apache-2.0

// Package handlers provides HTTP handlers for CB transfer limit management (R1-10.1).
package handlers

import (
	"context"
	"errors"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// TransferLimitServiceIface is the persistence interface consumed by TransferLimitHandler.
type TransferLimitServiceIface interface {
	Create(ctx context.Context, centralBankID, participantID, currency, maxAmount string) (*domain.TransferLimit, error)
	List(ctx context.Context, centralBankID string) ([]domain.TransferLimit, error)
	Delete(ctx context.Context, limitID, centralBankID string) error
}

// TransferLimitHandler handles /api/v2/governance/transfer-limits endpoints.
type TransferLimitHandler struct {
	svc TransferLimitServiceIface
}

// NewTransferLimitHandler creates a TransferLimitHandler.
func NewTransferLimitHandler(svc TransferLimitServiceIface) *TransferLimitHandler {
	return &TransferLimitHandler{svc: svc}
}

// createTransferLimitRequest is the JSON body for POST /api/v2/governance/transfer-limits.
type createTransferLimitRequest struct {
	ParticipantID string `json:"participant_id"`
	Currency      string `json:"currency"`
	// MaxAmount is a human-readable decimal string (e.g. "100000.00").
	// Converted to wei internally before persistence.
	MaxAmount string `json:"max_amount"`
}

// CreateTransferLimit handles POST /api/v2/governance/transfer-limits.
// Requires central_bank role. Sets a daily transfer limit for a participant and/or currency.
func (h *TransferLimitHandler) CreateTransferLimit(c *fiber.Ctx) error {
	cbID := centralBankIDFromClaims(c)
	if cbID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "central bank identity required"})
	}

	var req createTransferLimitRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.MaxAmount == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "max_amount required"})
	}

	if req.ParticipantID != "" && !sameSuffix(cbID, req.ParticipantID) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error":      "participant does not belong to your spoke",
			"error_code": "PARTICIPANT_SPOKE_MISMATCH",
		})
	}

	limit, err := h.svc.Create(c.Context(), cbID, req.ParticipantID, req.Currency, req.MaxAmount)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(limit)
}

// ListTransferLimits handles GET /api/v2/governance/transfer-limits.
// Returns all active limits created by the authenticated CB.
func (h *TransferLimitHandler) ListTransferLimits(c *fiber.Ctx) error {
	cbID := centralBankIDFromClaims(c)
	if cbID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "central bank identity required"})
	}

	limits, err := h.svc.List(c.Context(), cbID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"limits": limits})
}

// DeleteTransferLimit handles DELETE /api/v2/governance/transfer-limits/:id.
// Only the CB that created the limit may delete it.
func (h *TransferLimitHandler) DeleteTransferLimit(c *fiber.Ctx) error {
	cbID := centralBankIDFromClaims(c)
	if cbID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "central bank identity required"})
	}

	limitID := c.Params("id")
	if limitID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "limit id required"})
	}

	if err := h.svc.Delete(c.Context(), limitID, cbID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "limit not found"})
		}
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// centralBankIDFromClaims extracts the bank_id from JWT claims.
func centralBankIDFromClaims(c *fiber.Ctx) string {
	claims, ok := c.Locals("claims").(domain.TokenClaims)
	if !ok {
		return ""
	}
	return strings.TrimSpace(claims.BankID)
}

// sameSuffix reports whether two bank codes share the same spoke suffix ("-a" or "-b").
func sameSuffix(cbID, participantID string) bool {
	suffix := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		if strings.HasSuffix(s, "-a") {
			return "a"
		}
		if strings.HasSuffix(s, "-b") {
			return "b"
		}
		return ""
	}
	return suffix(cbID) == suffix(participantID) && suffix(cbID) != ""
}

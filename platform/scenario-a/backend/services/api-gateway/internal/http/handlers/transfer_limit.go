package handlers

import (
	"context"

	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	"github.com/gofiber/fiber/v2"
)

// transferLimitManager is satisfied by *complianceadapter.GRPCAdapter.
type transferLimitManager interface {
	CreateTransferLimit(ctx context.Context, participantID, currency, maxAmount, actorSubject string) (complianceadapter.TransferLimit, error)
	ListTransferLimits(ctx context.Context, centralBankID string) ([]complianceadapter.TransferLimit, error)
	DeleteTransferLimit(ctx context.Context, limitID, actorSubject string) error
}

// TransferLimitHandler handles Treasury-portal endpoints for R1-10.1 (CB only).
type TransferLimitHandler struct {
	mgr transferLimitManager
}

// NewTransferLimitHandler creates a new TransferLimitHandler.
func NewTransferLimitHandler(mgr transferLimitManager) *TransferLimitHandler {
	return &TransferLimitHandler{mgr: mgr}
}

// CreateTransferLimit handles POST /api/v1/treasury/transfer-limits.
func (h *TransferLimitHandler) CreateTransferLimit(c *fiber.Ctx) error {
	var req struct {
		ParticipantID string `json:"participant_id"`
		Currency      string `json:"currency"`
		MaxAmount     string `json:"max_amount"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.MaxAmount == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "max_amount is required"})
	}

	actor := actorFromClaims(c)
	limit, err := h.mgr.CreateTransferLimit(c.Context(), req.ParticipantID, req.Currency, req.MaxAmount, actor)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(limit)
}

// ListTransferLimits handles GET /api/v1/treasury/transfer-limits.
func (h *TransferLimitHandler) ListTransferLimits(c *fiber.Ctx) error {
	cbID := c.Query("central_bank_id", "")
	limits, err := h.mgr.ListTransferLimits(c.Context(), cbID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if limits == nil {
		limits = []complianceadapter.TransferLimit{}
	}
	return c.JSON(fiber.Map{"limits": limits})
}

// DeleteTransferLimit handles DELETE /api/v1/treasury/transfer-limits/:id.
func (h *TransferLimitHandler) DeleteTransferLimit(c *fiber.Ctx) error {
	limitID := c.Params("id")
	if limitID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "id is required"})
	}
	actor := actorFromClaims(c)
	if err := h.mgr.DeleteTransferLimit(c.Context(), limitID, actor); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

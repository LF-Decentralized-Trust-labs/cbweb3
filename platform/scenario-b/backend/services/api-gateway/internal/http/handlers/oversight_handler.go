// Package handlers provides oversight HTTP handlers for Master Viewing Key disclosure (FR-034/FR-035/FR-036).
package handlers

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

// OversightServiceIface is the interface consumed by OversightHandler.
type OversightServiceIface interface {
	OpenDisclosure(ctx context.Context, txRef, requestorID, reasonCode string) (*services.DisclosureResult, error)
	SignDisclosure(ctx context.Context, requestID, signerID string) error
	GetDisclosureStatus(ctx context.Context, requestID string) (*services.DisclosureResult, error)
}

// OversightHandler handles oversight disclosure endpoints.
type OversightHandler struct {
	svc OversightServiceIface
}

// NewOversightHandler creates an OversightHandler.
func NewOversightHandler(svc OversightServiceIface) *OversightHandler {
	return &OversightHandler{svc: svc}
}

// OpenDisclosure handles POST /api/v2/oversight/disclosure-request.
func (h *OversightHandler) OpenDisclosure(c *fiber.Ctx) error {
	var req struct {
		TxRef       string `json:"tx_ref"`
		RequestorID string `json:"requestor_id"`
		ReasonCode  string `json:"reason_code"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.TxRef == "" || req.RequestorID == "" || req.ReasonCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tx_ref, requestor_id, reason_code required"})
	}

	result, err := h.svc.OpenDisclosure(c.Context(), req.TxRef, req.RequestorID, req.ReasonCode)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

// SignDisclosure handles POST /api/v2/oversight/disclosure-sign.
func (h *OversightHandler) SignDisclosure(c *fiber.Ctx) error {
	var req struct {
		RequestID string `json:"request_id"`
		SignerID  string `json:"signer_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if err := h.svc.SignDisclosure(c.Context(), req.RequestID, req.SignerID); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "signed"})
}

// GetDisclosureStatus handles GET /api/v2/oversight/disclosure-status/:requestID.
func (h *OversightHandler) GetDisclosureStatus(c *fiber.Ctx) error {
	requestID := c.Params("requestID")
	if requestID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "requestID required"})
	}

	result, err := h.svc.GetDisclosureStatus(c.Context(), requestID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(result)
}

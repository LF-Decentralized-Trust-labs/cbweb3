package handlers

import (
	"context"
	"log"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

// OversightServiceIface is the contract the handler requires from its service.
type OversightServiceIface interface {
	OpenDisclosure(ctx context.Context, txRef, requestorID, reasonCode string) (*services.DisclosureResult, error)
	SignDisclosure(ctx context.Context, requestID, signerID string) error
	GetDisclosureStatus(ctx context.Context, requestID string) (*services.DisclosureResult, error)
}

// OversightHandler exposes the Investigation Module endpoints (FR-034/FR-035/FR-036).
type OversightHandler struct {
	svc OversightServiceIface
}

// NewOversightHandler creates an OversightHandler.
func NewOversightHandler(svc OversightServiceIface) *OversightHandler {
	return &OversightHandler{svc: svc}
}

// OpenDisclosure handles POST /api/v1/oversight/disclosure-request.
// Returns 201 with the new request on success, 400 on validation failure.
func (h *OversightHandler) OpenDisclosure(c *fiber.Ctx) error {
	var body struct {
		TxRef       string `json:"tx_ref"`
		RequestorID string `json:"requestor_id"`
		ReasonCode  string `json:"reason_code"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if body.TxRef == "" || body.RequestorID == "" || body.ReasonCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tx_ref, requestor_id, and reason_code are required"})
	}
	if !services.IsValidReasonCode(body.ReasonCode) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid reason_code; accepted: AML_ALERT, CFT_INVESTIGATION, COURT_ORDER, REGULATORY_EXAM"})
	}

	result, err := h.svc.OpenDisclosure(c.UserContext(), body.TxRef, body.RequestorID, body.ReasonCode)
	if err != nil {
		log.Printf("[oversight] OpenDisclosure failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(disclosureResultJSON(result))
}

// SignDisclosure handles POST /api/v1/oversight/disclosure-sign.
// Returns 200 on success, 400 when a duplicate signature or invalid state is detected.
func (h *OversightHandler) SignDisclosure(c *fiber.Ctx) error {
	var body struct {
		RequestID string `json:"request_id"`
		SignerID  string `json:"signer_id"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if body.RequestID == "" || body.SignerID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "request_id and signer_id are required"})
	}

	if err := h.svc.SignDisclosure(c.UserContext(), body.RequestID, body.SignerID); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "already signed") || strings.Contains(msg, "not in PENDING") || strings.Contains(msg, "expired") {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": msg})
		}
		if strings.Contains(msg, "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": msg})
		}
		log.Printf("[oversight] SignDisclosure failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": msg})
	}
	return c.JSON(fiber.Map{"status": "signed"})
}

// GetDisclosureStatus handles GET /api/v1/oversight/disclosure-status/:requestID.
// Returns 200 with the request detail, 404 when not found.
func (h *OversightHandler) GetDisclosureStatus(c *fiber.Ctx) error {
	requestID := c.Params("requestID")
	if requestID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "requestID is required"})
	}

	result, err := h.svc.GetDisclosureStatus(c.UserContext(), requestID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}
		log.Printf("[oversight] GetDisclosureStatus failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(disclosureResultJSON(result))
}

func disclosureResultJSON(r *services.DisclosureResult) fiber.Map {
	var closedAt any
	if r.ClosedAt != nil {
		closedAt = r.ClosedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	return fiber.Map{
		"request_id":             r.RequestID,
		"requested_by_bank_id":   r.RequestedByBankID,
		"target_transaction_ref": r.TargetTransactionRef,
		"reason_code":            r.ReasonCode,
		"state":                  r.State,
		"quorum_required":        r.QuorumRequired,
		"quorum_reached":         r.QuorumReached,
		"opened_at":              r.OpenedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		"expires_at":             r.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		"closed_at":              closedAt,
	}
}

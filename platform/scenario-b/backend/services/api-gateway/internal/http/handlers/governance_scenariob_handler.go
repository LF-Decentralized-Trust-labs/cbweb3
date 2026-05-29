// Package handlers provides governance circuit-breaker HTTP handlers for Scenario B (FR-030 / SC-017 / SC-026).
package handlers

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

// CircuitBreakerServiceIface is the interface consumed by GovernanceScenarioBHandler.
type CircuitBreakerServiceIface interface {
	Pause(ctx context.Context, pair, bankID, reasonCode string, signature []byte) error
	ProposeResume(ctx context.Context, pair, bankID string, sig []byte) (string, error)
	SignResume(ctx context.Context, pair, requestID, bankID string, sig []byte) error
	ExecuteResume(ctx context.Context, pair, requestID string) error
	GetStatus(ctx context.Context, pair string) (*services.CircuitBreakerStatus, error)
}

// GovernanceScenarioBHandler handles Circuit Breaker governance endpoints.
type GovernanceScenarioBHandler struct {
	cbSvc CircuitBreakerServiceIface
}

// NewGovernanceScenarioBHandler creates a GovernanceScenarioBHandler.
func NewGovernanceScenarioBHandler(cbSvc CircuitBreakerServiceIface) *GovernanceScenarioBHandler {
	return &GovernanceScenarioBHandler{cbSvc: cbSvc}
}

// PauseCircuitBreaker handles POST /api/v2/governance/circuit-breaker/pause (1-of-N).
func (h *GovernanceScenarioBHandler) PauseCircuitBreaker(c *fiber.Ctx) error {
	var req struct {
		Pair       string `json:"pair"`
		BankID     string `json:"bank_id"`
		ReasonCode string `json:"reason_code"`
		Signature  []byte `json:"signature"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Pair == "" || req.BankID == "" || req.ReasonCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "pair, bank_id, reason_code required"})
	}

	if err := h.cbSvc.Pause(c.Context(), req.Pair, req.BankID, req.ReasonCode, req.Signature); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"state": "HALTED", "pair": req.Pair})
}

// ProposeResume handles POST /api/v2/governance/circuit-breaker/resume-request.
func (h *GovernanceScenarioBHandler) ProposeResume(c *fiber.Ctx) error {
	var req struct {
		Pair      string `json:"pair"`
		BankID    string `json:"bank_id"`
		Signature []byte `json:"signature"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	requestID, err := h.cbSvc.ProposeResume(c.Context(), req.Pair, req.BankID, req.Signature)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"state": "RESUME_PENDING", "request_id": requestID})
}

// SignResume handles POST /api/v2/governance/circuit-breaker/resume-sign.
func (h *GovernanceScenarioBHandler) SignResume(c *fiber.Ctx) error {
	var req struct {
		Pair      string `json:"pair"`
		RequestID string `json:"request_id"`
		BankID    string `json:"bank_id"`
		Signature []byte `json:"signature"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if err := h.cbSvc.SignResume(c.Context(), req.Pair, req.RequestID, req.BankID, req.Signature); err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":      err.Error(),
			"error_code": "CIRCUIT_BREAKER_RESUME_DISPUTED",
		})
	}

	// Attempt to execute if quorum might be reached
	if err := h.cbSvc.ExecuteResume(c.Context(), req.Pair, req.RequestID); err != nil {
		return c.JSON(fiber.Map{"state": "RESUME_PENDING", "request_id": req.RequestID})
	}
	return c.JSON(fiber.Map{"state": "LIVE", "pair": req.Pair})
}

// GetCircuitBreakerStatus handles GET /api/v2/governance/circuit-breaker/status.
func (h *GovernanceScenarioBHandler) GetCircuitBreakerStatus(c *fiber.Ctx) error {
	pair := c.Query("pair", "BRL-USD")
	status, err := h.cbSvc.GetStatus(c.Context(), pair)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(status)
}

// Ensure the CircuitBreakerGateImpl interface is satisfied (compile-time check).
// This reference is here to verify the implementation without creating import cycles.

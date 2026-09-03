// SPDX-License-Identifier: Apache-2.0

// Package handlers provides governance circuit-breaker HTTP handlers for Scenario B (FR-030 / SC-017 / SC-026).
package handlers

import (
	"context"
	"log"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

// CircuitBreakerServiceIface is the interface consumed by GovernanceScenarioBHandler.
// The three write actions return the hash of the on-chain transaction they submitted;
// ExecuteResume makes no chain call and so has none.
type CircuitBreakerServiceIface interface {
	Pause(ctx context.Context, pair, bankID, reasonCode string, signature []byte) (string, error)
	ProposeResume(ctx context.Context, pair, bankID string, sig []byte) (requestID string, txHash string, err error)
	SignResume(ctx context.Context, pair, requestID, bankID string, sig []byte) (txHash string, err error)
	ExecuteResume(ctx context.Context, pair, requestID string) error
	GetStatus(ctx context.Context, pair string) (*services.CircuitBreakerStatus, error)
}

// withTxHash adds the on-chain reference to a response body only when one exists. The key is
// omitted rather than set to "" (contract rule C-1 / FR-009): an empty string would be
// indistinguishable from "a reference exists but was not returned", whereas an absent key
// says plainly that this environment recorded no on-chain reference for the action.
func withTxHash(body fiber.Map, txHash string) fiber.Map {
	if txHash != "" {
		body["tx_hash"] = txHash
	}
	return body
}

// CircuitBreakerSigner produces the off-chain institutional attestation for a circuit
// breaker action, signed with this Central Bank's own key. Implemented by relayauth.Signer.
type CircuitBreakerSigner interface {
	SignAttestation(message string) ([]byte, error)
}

// GovernanceScenarioBHandler handles Circuit Breaker governance endpoints.
type GovernanceScenarioBHandler struct {
	cbSvc  CircuitBreakerServiceIface
	signer CircuitBreakerSigner
}

// NewGovernanceScenarioBHandler creates a GovernanceScenarioBHandler.
func NewGovernanceScenarioBHandler(cbSvc CircuitBreakerServiceIface) *GovernanceScenarioBHandler {
	return &GovernanceScenarioBHandler{cbSvc: cbSvc}
}

// WithSigner attaches the CB key so the handler generates the institutional attestation
// server-side (operators never supply a signature). When nil, the attestation is empty —
// on-chain authorization is unaffected (it is the signer address / msg.sender).
func (h *GovernanceScenarioBHandler) WithSigner(s CircuitBreakerSigner) *GovernanceScenarioBHandler {
	h.signer = s
	return h
}

// attest builds the canonical action message and signs it with the CB key. Returns nil
// (and logs) when no signer is configured or signing fails — the action still proceeds;
// on-chain auth does not depend on this blob.
func (h *GovernanceScenarioBHandler) attest(parts ...string) []byte {
	if h.signer == nil {
		return nil
	}
	msg := "circuit-breaker"
	for _, p := range parts {
		msg += "|" + p
	}
	sig, err := h.signer.SignAttestation(msg)
	if err != nil {
		log.Printf("[circuit-breaker] attestation signing failed: %v", err)
		return nil
	}
	return sig
}

// PauseCircuitBreaker handles POST /api/v2/governance/circuit-breaker/pause (1-of-N).
func (h *GovernanceScenarioBHandler) PauseCircuitBreaker(c *fiber.Ctx) error {
	var req struct {
		Pair       string `json:"pair"`
		BankID     string `json:"bank_id"`
		ReasonCode string `json:"reason_code"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Pair == "" || req.BankID == "" || req.ReasonCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "pair, bank_id, reason_code required"})
	}

	// Institutional attestation is generated server-side with the CB key (never supplied
	// by the client). On-chain authorization is the gateway signer address.
	signature := h.attest("pause", req.Pair, req.BankID, req.ReasonCode)
	txHash, err := h.cbSvc.Pause(c.Context(), req.Pair, req.BankID, req.ReasonCode, signature)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(withTxHash(fiber.Map{"state": "HALTED", "pair": req.Pair}, txHash))
}

// ProposeResume handles POST /api/v2/governance/circuit-breaker/resume-request.
func (h *GovernanceScenarioBHandler) ProposeResume(c *fiber.Ctx) error {
	var req struct {
		Pair   string `json:"pair"`
		BankID string `json:"bank_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	signature := h.attest("resume-propose", req.Pair, req.BankID)
	// request_id and tx_hash are different identifiers and both are returned: request_id is
	// the proposal a co-signer must submit to resume-sign, tx_hash is the audit reference for
	// the proposing transaction. Substituting one for the other would break signing.
	requestID, txHash, err := h.cbSvc.ProposeResume(c.Context(), req.Pair, req.BankID, signature)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(withTxHash(fiber.Map{"state": "RESUME_PENDING", "request_id": requestID}, txHash))
}

// SignResume handles POST /api/v2/governance/circuit-breaker/resume-sign.
func (h *GovernanceScenarioBHandler) SignResume(c *fiber.Ctx) error {
	var req struct {
		Pair      string `json:"pair"`
		RequestID string `json:"request_id"`
		BankID    string `json:"bank_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	signature := h.attest("resume-sign", req.Pair, req.RequestID, req.BankID)
	txHash, err := h.cbSvc.SignResume(c.Context(), req.Pair, req.RequestID, req.BankID, signature)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":      err.Error(),
			"error_code": "CIRCUIT_BREAKER_RESUME_DISPUTED",
		})
	}

	// Attempt to execute if quorum might be reached. Either way the signing transaction has
	// landed, so its hash is reported in both the pending and the resumed response.
	if err := h.cbSvc.ExecuteResume(c.Context(), req.Pair, req.RequestID); err != nil {
		return c.JSON(withTxHash(fiber.Map{"state": "RESUME_PENDING", "request_id": req.RequestID}, txHash))
	}
	return c.JSON(withTxHash(fiber.Map{"state": "LIVE", "pair": req.Pair}, txHash))
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

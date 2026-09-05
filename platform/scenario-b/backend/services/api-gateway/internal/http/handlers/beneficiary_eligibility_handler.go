// SPDX-License-Identifier: Apache-2.0

// Package handlers provides the beneficiary eligibility check a central bank answers about its
// own member banks.
//
// POST /internal/amm/beneficiary-eligibility   {"bank_id": "<code>"}
//
// A POST for a read, deliberately. The relay signature covers the method, the path and the
// BODY — never the query string, because the verifier reconstructs the path with Fiber's
// c.Path(), which drops it. Passing bank_id in the query would leave the one parameter that
// selects the answer unsigned, so a captured signature could be replayed against a different
// bank. In the body it is covered by the same signature as every other internal call.
//
// Asked by another central bank, through the Cacti relay, BEFORE it moves any value. It exists
// because the condition that rejects a delivery is known only here — the participants registry
// belongs to this central bank — and was previously consulted only at the delivery itself,
// which is after bridge-in and after the AMM swap, i.e. past the point of no return.
//
// Sovereignty shapes the response, not just the auth. The answer is {eligible, code} and
// nothing more: no wallet address, no status string, no registry row. Another central bank
// needs to know whether to start a payment, not who the beneficiary is or how far through
// onboarding it got. Address resolution stays inside the bridge-out, on this CB's own side,
// exactly as before.
//
// Protected by the same X-Relay-Auth middleware as the other internal endpoints.
package handlers

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

// BeneficiaryEligibilityCheckerIface answers whether a member bank can receive a delivery.
type BeneficiaryEligibilityCheckerIface interface {
	CheckBeneficiaryEligibility(ctx context.Context, bankCode string) (services.BeneficiaryEligibility, error)
}

// BeneficiaryEligibilityHandler serves the pre-flight check.
type BeneficiaryEligibilityHandler struct {
	checker BeneficiaryEligibilityCheckerIface
}

// NewBeneficiaryEligibilityHandler constructs the handler.
func NewBeneficiaryEligibilityHandler(checker BeneficiaryEligibilityCheckerIface) *BeneficiaryEligibilityHandler {
	return &BeneficiaryEligibilityHandler{checker: checker}
}

// HandleCheck answers the asking central bank.
//
// An ineligible beneficiary is 200 with eligible=false, NOT an error status. The question was
// answered; the answer is no. Reserving non-2xx for "could not answer" is what lets the caller
// tell a definite refusal apart from an unreachable peer — and those two must lead to different
// decisions, because only one of them is safe to treat as a reason to stop.
func (h *BeneficiaryEligibilityHandler) HandleCheck(c *fiber.Ctx) error {
	var req struct {
		BankID string `json:"bank_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	bankID := req.BankID
	if bankID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "bank_id is required",
		})
	}
	if h.checker == nil {
		// Fail loudly rather than answering "eligible" on a gateway that cannot check: a
		// confident yes from a CB that never looked is the one answer that could authorise the
		// payment this endpoint exists to prevent.
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "participant registry not configured on this gateway",
		})
	}

	result, err := h.checker.CheckBeneficiaryEligibility(c.Context(), bankID)
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "could not check beneficiary eligibility: " + err.Error(),
		})
	}
	return c.JSON(fiber.Map{
		"eligible": result.Eligible,
		"code":     result.Code,
	})
}

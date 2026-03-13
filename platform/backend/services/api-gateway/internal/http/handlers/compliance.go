// This file handles compliance HTTP endpoints such as KYC status lookup.
package handlers

import (
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/interfaces"
	"github.com/gofiber/fiber/v2"
)

// ComplianceHandler exposes compliance-related HTTP endpoints.
type ComplianceHandler struct {
	kyc interfaces.KYCChecker
}

// NewComplianceHandler builds a ComplianceHandler.
func NewComplianceHandler(kyc interfaces.KYCChecker) *ComplianceHandler {
	return &ComplianceHandler{kyc: kyc}
}

// GetKYCStatus returns the KYC status for the requested subject.
func (h *ComplianceHandler) GetKYCStatus(c *fiber.Ctx) error {
	subject := c.Params("subject")
	if subject == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "subject is required"})
	}
	return c.JSON(fiber.Map{
		"subject": subject,
		"status":  h.kyc.GetStatus(subject),
	})
}


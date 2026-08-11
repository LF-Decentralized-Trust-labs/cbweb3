// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"log"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

// HubReconcilerIface reconciles this central bank's own Hub W-token balance against its records.
type HubReconcilerIface interface {
	Reconcile(ctx context.Context) (services.ReconciliationReport, error)
}

// HubReconciliationHandler serves GET /api/v2/amm/hub-reconciliation.
type HubReconciliationHandler struct {
	reconciler HubReconcilerIface
}

// NewHubReconciliationHandler constructs the handler.
func NewHubReconciliationHandler(r HubReconcilerIface) *HubReconciliationHandler {
	return &HubReconciliationHandler{reconciler: r}
}

// GetReconciliation reports what this CB holds on the Hub for its banks, and what part of it its
// own records cannot account for.
//
// The balance is an omnibus one: the banks never hold W-token, so this is the CB's obligation
// toward banks whose reserves were consumed and whose payments have not closed. At rest it should
// be zero, which makes `unexplained` the number that matters — value on-chain that this CB cannot
// attribute to a payment.
//
// A read failure is an error, never an empty report: answering "balanced" because the chain or the
// database could not be read would be the one outcome worse than an unexplained balance.
func (h *HubReconciliationHandler) GetReconciliation(c *fiber.Ctx) error {
	if h.reconciler == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "hub reconciliation is not configured on this gateway (needs HUB_BESU_RPC_URL, W_TOKEN_ADDRESS and this CB's hub address)",
			"code":  "RECONCILIATION_NOT_CONFIGURED",
		})
	}
	report, err := h.reconciler.Reconcile(c.Context())
	if err != nil {
		// The cause goes to the log, not to the response: the underlying error carries the Hub RPC
		// endpoint and driver-level database text, and this endpoint is reachable by any
		// authenticated CB operator. The caller gets the code it can act on.
		log.Printf("[hub-reconciliation] reconciliation failed: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "reconciliation failed — see gateway logs",
			"code":  "RECONCILIATION_FAILED",
		})
	}
	return c.JSON(report)
}

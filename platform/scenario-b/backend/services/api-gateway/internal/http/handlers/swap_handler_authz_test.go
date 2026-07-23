// SPDX-License-Identifier: Apache-2.0

// Route-authorization tests for the AMM swap handler (R2-H-9 / R2-H-10): the
// payer identity must be derived from the authenticated JWT (claims.BankID), not
// trusted from the request body. A swap whose payer_id diverges from the caller's
// authenticated identity must be rejected.
package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// injectBankClaims installs a middleware that sets authenticated claims with the
// given BankID, mimicking the auth gateway.
func injectBankClaims(bankID string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{Subject: "user-1", BankID: bankID})
		return c.Next()
	}
}

func TestSwapHandler_PayerMismatchRejected(t *testing.T) {
	svc := &mockSwapService{resp: &services.SwapResult{OrderID: "ORD-001", State: "COMPLETED"}}
	h := handlers.NewSwapHandler(svc)

	app := fiber.New()
	// Authenticated as bank-x, but the body claims payer_id = bank-a.
	app.Post("/api/v2/amm/swap/exact-output", injectBankClaims("bank-x"), h.SwapExactOutput)

	req := httptest.NewRequest(http.MethodPost, "/api/v2/amm/swap/exact-output",
		bytes.NewReader(validSwapBody()))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestSwapHandler_PayerMatchesClaims(t *testing.T) {
	svc := &mockSwapService{resp: &services.SwapResult{OrderID: "ORD-001", State: "COMPLETED"}}
	h := handlers.NewSwapHandler(svc)

	app := fiber.New()
	// Authenticated as bank-a, matching the body payer_id.
	app.Post("/api/v2/amm/swap/exact-output", injectBankClaims("bank-a"), h.SwapExactOutput)

	req := httptest.NewRequest(http.MethodPost, "/api/v2/amm/swap/exact-output",
		bytes.NewReader(validSwapBody()))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ORD-001", body["order_id"])
}

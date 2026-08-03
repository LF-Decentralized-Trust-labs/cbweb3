// SPDX-License-Identifier: Apache-2.0

package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/gofiber/fiber/v2"
)

// The listing handlers are shared between two route families: the central bank's own portal, where
// listing every record is the point, and /internal/v1/payments/*, which serves one commercial bank.
// The handler filters by requester_id and returns everything when it is absent, so on the bank-facing
// family an absent parameter is not a broad query — it is a tenant boundary that was never applied.
//
// The bank's proxy always sets it now. This middleware makes the central bank stop depending on that:
// a caller that omits it is refused rather than answered with every bank's records.

func scopeApp(t *testing.T) *fiber.App {
	t.Helper()
	app := fiber.New()
	app.Get("/internal/v1/payments/deposits", middleware.RequireRequesterScope(), func(c *fiber.Ctx) error {
		return c.SendString("listed")
	})
	return app
}

func TestRequireRequesterScope_RefusesAnUnscopedListing(t *testing.T) {
	resp, err := scopeApp(t).Test(httptest.NewRequest(http.MethodGet, "/internal/v1/payments/deposits", nil), -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d; want 400 — without a scope the handler returns every bank's records", resp.StatusCode)
	}
}

func TestRequireRequesterScope_RefusesABlankRequesterID(t *testing.T) {
	resp, err := scopeApp(t).Test(httptest.NewRequest(http.MethodGet, "/internal/v1/payments/deposits?requester_id=%20%20", nil), -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d; want 400 — whitespace filters nothing", resp.StatusCode)
	}
}

func TestRequireRequesterScope_AllowsAScopedListing(t *testing.T) {
	resp, err := scopeApp(t).Test(
		httptest.NewRequest(http.MethodGet, "/internal/v1/payments/deposits?requester_id=0xITAU", nil), -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d; want 200", resp.StatusCode)
	}
}

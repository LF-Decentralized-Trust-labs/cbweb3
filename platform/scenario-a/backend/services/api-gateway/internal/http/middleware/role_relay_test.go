// SPDX-License-Identifier: Apache-2.0

// This file tests RequireRole and RequireRelayAuth middleware, plus the
// invalid-token branch of RequireCookieAuth.
package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/gofiber/fiber/v2"
)

func okHandler(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }

func TestRequireRole(t *testing.T) {
	t.Parallel()

	// Allowed: caller has the required role.
	app := fiber.New()
	app.Get("/x", func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{Subject: "u", Roles: []string{domain.RoleGovernance}})
		return c.Next()
	}, RequireRole(domain.RoleGovernance), okHandler)
	if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil)); resp.StatusCode != http.StatusOK {
		t.Errorf("allowed: want 200, got %d", resp.StatusCode)
	}

	// Forbidden: caller lacks the role.
	app2 := fiber.New()
	app2.Get("/x", func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{Subject: "u", Roles: []string{domain.RoleCommercialBank}})
		return c.Next()
	}, RequireRole(domain.RoleGovernance), okHandler)
	if resp, _ := app2.Test(httptest.NewRequest(http.MethodGet, "/x", nil)); resp.StatusCode != http.StatusForbidden {
		t.Errorf("forbidden: want 403, got %d", resp.StatusCode)
	}

	// Unauthorized: no claims in context.
	app3 := fiber.New()
	app3.Get("/x", RequireRole(domain.RoleGovernance), okHandler)
	if resp, _ := app3.Test(httptest.NewRequest(http.MethodGet, "/x", nil)); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no claims: want 401, got %d", resp.StatusCode)
	}
}

func TestRequireRelayAuth(t *testing.T) {
	t.Parallel()

	// Not configured (empty secret) → 503.
	app := fiber.New()
	app.Get("/x", RequireRelayAuth(""), okHandler)
	if resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/x", nil)); resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("unconfigured: want 503, got %d", resp.StatusCode)
	}

	// Missing header → 401.
	app2 := fiber.New()
	app2.Get("/x", RequireRelayAuth("secret"), okHandler)
	if resp, _ := app2.Test(httptest.NewRequest(http.MethodGet, "/x", nil)); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("missing header: want 401, got %d", resp.StatusCode)
	}

	// Wrong secret → 401.
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Relay-Auth", "wrong")
	if resp, _ := app2.Test(req); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong secret: want 401, got %d", resp.StatusCode)
	}

	// Correct secret → 200.
	req2 := httptest.NewRequest(http.MethodGet, "/x", nil)
	req2.Header.Set("X-Relay-Auth", "secret")
	if resp, _ := app2.Test(req2); resp.StatusCode != http.StatusOK {
		t.Errorf("correct secret: want 200, got %d", resp.StatusCode)
	}
}

func TestRequireCookieAuth_InvalidToken(t *testing.T) {
	t.Parallel()
	app := fiber.New()
	app.Get("/x", RequireCookieAuth(tokenValidatorStub{err: errors.New("invalid")}), okHandler)
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "bad"})
	if resp, _ := app.Test(req); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("invalid token: want 401, got %d", resp.StatusCode)
	}
}

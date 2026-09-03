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

// Reported from the Peru environment: the supervisor portal was reachable from an active
// treasury session. The portal was at fault — it admitted a session this middleware refuses —
// but the refusal is what kept it from being an authorization defect, so it is pinned here by
// the exact pairing rather than only by the generic case above.
//
// Confirmed live on a Scenario A stack: a ROLE_TREASURY session gets 403 from
// /compliance/audit/logs, /compliance/participants/summary and /oversight/network, while a
// ROLE_SUPERVISOR session gets 200 on the same route.
func TestRequireSupervisorRoleRefusesATreasurySession(t *testing.T) {
	t.Parallel()

	withRoles := func(roles ...string) *fiber.App {
		app := fiber.New()
		app.Get("/x", func(c *fiber.Ctx) error {
			c.Locals("claims", domain.TokenClaims{Subject: "operator", Roles: roles})
			return c.Next()
		}, RequireSupervisorRole(), okHandler)
		return app
	}

	// The reported case: treasury reaching a supervisor route.
	resp, _ := withRoles(domain.RoleTreasury).Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("treasury session: want 403, got %d", resp.StatusCode)
	}

	// Neighbouring roles must not slip through either.
	for _, role := range []string{domain.RoleGovernance, domain.RoleCommercialBank} {
		resp, _ := withRoles(role).Test(httptest.NewRequest(http.MethodGet, "/x", nil))
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s session: want 403, got %d", role, resp.StatusCode)
		}
	}

	// The control: the role the route is for still passes.
	resp, _ = withRoles(domain.RoleSupervisor).Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if resp.StatusCode != http.StatusOK {
		t.Errorf("supervisor session: want 200, got %d", resp.StatusCode)
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

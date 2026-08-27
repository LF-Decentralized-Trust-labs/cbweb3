// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/backend/services/noc-backend/internal/keycloak"
)

// claimsWithRoles builds the claim set RequireAuth would have stored.
func claimsWithRoles(roles []string) keycloak.TokenClaims {
	return keycloak.TokenClaims{Subject: "operator", Username: "operator", Roles: roles}
}

// The NOC portal-read routes carried RequireAuth alone. Their comment said "any NOC role",
// but nothing checked for one, so any authenticated token was served the dashboard — a
// treasury operator's included. Local stacks hide this: they run NOC_SKIP_AUTH=true, whose
// no-op Keycloak client returns all three NOC roles for any token at all.
//
// Found while fixing the portals that let a session in without checking its role; the NOC
// pair turned out to be the case where the backend really would have served the data.
func TestRequireAnyRoleGuardsThePortalGroup(t *testing.T) {
	t.Parallel()

	withRoles := func(roles ...string) *fiber.App {
		app := fiber.New()
		app.Get("/x", func(c *fiber.Ctx) error {
			c.Locals(claimsKey, claimsWithRoles(roles))
			return c.Next()
		}, RequireAnyRole(NOCPortalRoles...), func(c *fiber.Ctx) error {
			return c.SendStatus(fiber.StatusOK)
		})
		return app
	}

	// Every NOC role the deployment issues is admitted.
	for _, role := range NOCPortalRoles {
		resp, _ := withRoles(role).Test(httptest.NewRequest(http.MethodGet, "/x", nil))
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: want 200, got %d", role, resp.StatusCode)
		}
	}

	// Roles from other portals are not. These are the sessions that used to be served.
	for _, role := range []string{"ROLE_TREASURY", "ROLE_SUPERVISOR", "ROLE_GOVERNANCE"} {
		resp, _ := withRoles(role).Test(httptest.NewRequest(http.MethodGet, "/x", nil))
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s: want 403, got %d", role, resp.StatusCode)
		}
	}

	// No roles at all is a refusal, not a default-allow.
	resp, _ := withRoles().Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("no roles: want 403, got %d", resp.StatusCode)
	}

	// No claims in context means the auth middleware did not run: 401, not 403.
	app := fiber.New()
	app.Get("/x", RequireAnyRole(NOCPortalRoles...), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	resp, _ = app.Test(httptest.NewRequest(http.MethodGet, "/x", nil))
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no claims: want 401, got %d", resp.StatusCode)
	}
}

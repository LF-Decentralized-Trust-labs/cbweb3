// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/backend/services/noc-backend/internal/keycloak"
)

// These tests assert the WIRING, not the middleware.
//
// The authorization defect they guard against was never a bug in RequireRole or
// RequireAnyRole: both existed and worked. What was wrong is that the portal group was
// mounted with RequireAuth alone, under a comment that said "any NOC role" — so every
// authenticated token, a treasury operator's included, was served the dashboard and the
// pools data. A test that mounts the middleware itself passes either way and would not
// have caught it; removing the guard from the route group is the regression that matters,
// and only this level sees it.
//
// It is invisible on a running local stack for a separate reason: NOC backends deploy with
// NOC_SKIP_AUTH=true, and that no-op Keycloak client hands back all three NOC roles for
// any token at all.

// fakeKeycloak resolves a token string to the roles it carries. "bad" is rejected, which
// lets a test distinguish 401 (not authenticated) from 403 (authenticated, wrong role).
type fakeKeycloak struct {
	roles map[string][]string
}

func (f fakeKeycloak) ValidateToken(_ context.Context, token string) (keycloak.TokenClaims, error) {
	roles, ok := f.roles[token]
	if !ok {
		return keycloak.TokenClaims{}, fiber.ErrUnauthorized
	}
	return keycloak.TokenClaims{Subject: "sub-" + token, Username: token, Roles: roles}, nil
}

// probeHandler stands in for a real api handler: it mounts one route that answers 200, so
// any non-200 in these tests came from the group's middleware and not from the handler.
type probeHandler struct{ path string }

func (p probeHandler) Register(r fiber.Router) {
	r.Get(p.path, func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
}

// newTestApp builds the real wiring — the same mountRoutes main() calls — over probe
// handlers, so what is under test is the route groups and their middleware.
func newTestApp() *fiber.App {
	app := fiber.New()
	kc := fakeKeycloak{roles: map[string][]string{
		"noc-admin":    {"ROLE_NOC_ADMIN"},
		"noc-operator": {"ROLE_NOC_OPERATOR"},
		"noc-viewer":   {"ROLE_NOC_VIEWER"},
		"treasury":     {"ROLE_TREASURY"},
		"no-roles":     {},
	}}
	mountRoutes(app, kc, nocRoutes{
		Push:   probeHandler{path: "/api/v1/agents/push"},
		Admin:  []routeRegistrar{probeHandler{path: "/spokes"}},
		Portal: []routeRegistrar{probeHandler{path: "/pools"}},
	})
	return app
}

func status(t *testing.T, app *fiber.App, path, token string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request %s: %v", path, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

// The portal group must require a NOC role. A treasury token authenticates fine — it is a
// valid session — so the refusal has to come from the role check, which is why 403 and not
// 401 is the assertion that means the guard is wired.
func TestPortalGroupRequiresANOCRole(t *testing.T) {
	app := newTestApp()

	for _, token := range []string{"noc-admin", "noc-operator", "noc-viewer"} {
		if got := status(t, app, "/api/v1/pools", token); got != http.StatusOK {
			t.Errorf("%s must reach the portal group, got %d", token, got)
		}
	}

	if got := status(t, app, "/api/v1/pools", "treasury"); got != http.StatusForbidden {
		t.Errorf("a treasury session must be refused the NOC portal data, got %d.\n"+
			"If this is 200, the portal group lost its role guard and every authenticated "+
			"token is being served the dashboard again.", got)
	}
	if got := status(t, app, "/api/v1/pools", "no-roles"); got != http.StatusForbidden {
		t.Errorf("a token with no roles must be refused, got %d", got)
	}
	if got := status(t, app, "/api/v1/pools", ""); got != http.StatusUnauthorized {
		t.Errorf("an unauthenticated request must be 401, got %d", got)
	}
}

// The admin group is narrower than the portal group: a viewer or operator reads the
// dashboard but must not reach admin routes.
func TestAdminGroupRequiresTheAdminRole(t *testing.T) {
	app := newTestApp()

	if got := status(t, app, "/api/v1/admin/spokes", "noc-admin"); got != http.StatusOK {
		t.Errorf("ROLE_NOC_ADMIN must reach the admin group, got %d", got)
	}
	for _, token := range []string{"noc-operator", "noc-viewer", "treasury"} {
		if got := status(t, app, "/api/v1/admin/spokes", token); got != http.StatusForbidden {
			t.Errorf("%s must not reach the admin group, got %d", token, got)
		}
	}
	if got := status(t, app, "/api/v1/admin/spokes", ""); got != http.StatusUnauthorized {
		t.Errorf("an unauthenticated request must be 401, got %d", got)
	}
}

// The push endpoint authenticates agents by API key, not Keycloak. It must stay outside
// both groups: sweeping it behind the JWT middleware would stop every agent reporting.
func TestPushRouteStaysOutsideTheKeycloakGroups(t *testing.T) {
	app := newTestApp()

	if got := status(t, app, "/api/v1/agents/push", ""); got != http.StatusOK {
		t.Errorf("the agent push route must not sit behind Keycloak auth, got %d", got)
	}
}

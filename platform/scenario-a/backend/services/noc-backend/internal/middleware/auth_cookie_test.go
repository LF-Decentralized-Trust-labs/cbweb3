// SPDX-License-Identifier: Apache-2.0

package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/keycloak"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/middleware"
)

// RequireAuth used to read the Bearer header and nothing else, because the portal held its
// token in localStorage and attached it by hand. Moving the session into an HttpOnly cookie
// means the cookie is now the credential — and the Bearer path has to survive for exactly
// one caller: the toolkit, which provisions NOC agent keys during every deploy.

type authKCStub struct {
	accepted string
}

func (s *authKCStub) ValidateToken(_ context.Context, token string) (keycloak.TokenClaims, error) {
	if token != s.accepted {
		return keycloak.TokenClaims{}, keycloak.ErrInvalidCredentials
	}
	return keycloak.TokenClaims{Subject: "sub", Username: "noc-admin", Roles: []string{"ROLE_NOC_ADMIN"}}, nil
}

func (s *authKCStub) PasswordGrant(context.Context, string, string) (keycloak.Tokens, error) {
	return keycloak.Tokens{}, nil
}

func (s *authKCStub) RefreshGrant(context.Context, string) (keycloak.Tokens, error) {
	return keycloak.Tokens{}, nil
}

func authApp(kc keycloak.Client, path string) *fiber.App {
	app := fiber.New()
	app.Get(path, middleware.RequireAuth(kc), func(c *fiber.Ctx) error { return c.SendString("ok") })
	app.Post(path, middleware.RequireAuth(kc), func(c *fiber.Ctx) error { return c.SendString("ok") })
	return app
}

func statusFor(t *testing.T, app *fiber.App, method, path string, cookie, bearer string) int {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "access_token", Value: cookie})
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp.StatusCode
}

func TestRequireAuth_AcceptsTheSessionCookie(t *testing.T) {
	kc := &authKCStub{accepted: "good"}
	app := authApp(kc, "/api/v1/dashboard")

	if got := statusFor(t, app, http.MethodGet, "/api/v1/dashboard", "good", ""); got != http.StatusOK {
		t.Errorf("status = %d, want 200 — the cookie is the credential now", got)
	}
}

func TestRequireAuth_RejectsAnInvalidCookie(t *testing.T) {
	kc := &authKCStub{accepted: "good"}
	app := authApp(kc, "/api/v1/dashboard")

	if got := statusFor(t, app, http.MethodGet, "/api/v1/dashboard", "stale", ""); got != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", got)
	}
}

func TestRequireAuth_RejectsARequestWithNoCredentialAtAll(t *testing.T) {
	kc := &authKCStub{accepted: "good"}
	app := authApp(kc, "/api/v1/dashboard")

	if got := statusFor(t, app, http.MethodGet, "/api/v1/dashboard", "", ""); got != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", got)
	}
}

// TestRequireAuth_RefusesBearerOnAPortalRoute is the decision this card required, expressed
// as a test. Leaving Bearer acceptable everywhere would keep open the self-selected CSRF
// exemption the api-gateways just closed: a caller could waive the cookie and still be
// served. Only the machine path below is exempt.
func TestRequireAuth_RefusesBearerOnAPortalRoute(t *testing.T) {
	kc := &authKCStub{accepted: "good"}
	app := authApp(kc, "/api/v1/dashboard")

	if got := statusFor(t, app, http.MethodGet, "/api/v1/dashboard", "", "good"); got != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 — a valid Bearer must not authenticate a browser route", got)
	}
}

// TestRequireAuth_AcceptsBearerOnTheToolkitsProvisioningRoute keeps the deploy working. The
// toolkit posts here with a Bearer token on every entity it provisions
// (toolkit/engine/orchestrator/noc.go), and it is not a browser: it has no cookie to send.
func TestRequireAuth_AcceptsBearerOnTheToolkitsProvisioningRoute(t *testing.T) {
	kc := &authKCStub{accepted: "good"}
	app := authApp(kc, "/api/v1/admin/agents/provision-key")

	if got := statusFor(t, app, http.MethodPost, "/api/v1/admin/agents/provision-key", "", "good"); got != http.StatusOK {
		t.Errorf("status = %d, want 200 — removing this breaks NOC agent provisioning on every deploy", got)
	}
}

// TestRequireAuth_StillValidatesBearerOnTheMachineRoute: the exemption is about WHERE the
// credential may live, never about whether it is checked.
func TestRequireAuth_StillValidatesBearerOnTheMachineRoute(t *testing.T) {
	kc := &authKCStub{accepted: "good"}
	app := authApp(kc, "/api/v1/admin/agents/provision-key")

	if got := statusFor(t, app, http.MethodPost, "/api/v1/admin/agents/provision-key", "", "forged"); got != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for an invalid Bearer", got)
	}
}

// TestRequireAuth_PrefersTheCookieOnTheMachineRoute pins precedence. A browser operator
// using the admin screens sends a cookie; that must be the credential even on the one path
// where a Bearer is also accepted, so an attacker cannot downgrade a session by attaching a
// header of their own.
func TestRequireAuth_PrefersTheCookieOnTheMachineRoute(t *testing.T) {
	kc := &authKCStub{accepted: "good"}
	app := authApp(kc, "/api/v1/admin/agents/provision-key")

	if got := statusFor(t, app, http.MethodPost, "/api/v1/admin/agents/provision-key", "good", "forged"); got != http.StatusOK {
		t.Errorf("status = %d, want 200 — a valid cookie must win over a bad header", got)
	}
}

// The toolkit calls MORE than provision-key. register-noc-spoke posts to /admin/spokes and
// re-runs read /admin/spokes/<uuid> to decide whether it already registered. Allowing only
// provision-key left the deploy failing at 401 "no session" — the credential was fine, the
// path was not on the list.
func TestRequireAuth_AcceptsBearerOnEveryToolkitAdminPath(t *testing.T) {
	kc := &authKCStub{accepted: "good"}
	for _, path := range []string{
		"/api/v1/admin/spokes",
		"/api/v1/admin/spokes/2f1c6b7e-0000-0000-0000-000000000000",
		"/api/v1/admin/agents/provision-key",
	} {
		app := authApp(kc, path)
		if got := statusFor(t, app, http.MethodPost, path, "", "good"); got != http.StatusOK {
			t.Errorf("%s with a Bearer = %d, want 200 — the toolkit calls this during every deploy",
				path, got)
		}
	}
}

// The prefix must not become a wildcard over the admin tree: only the spokes surface the
// toolkit actually uses is machine-reachable.
func TestRequireAuth_BearerIsNotAllowedAcrossTheWholeAdminTree(t *testing.T) {
	kc := &authKCStub{accepted: "good"}
	for _, path := range []string{"/api/v1/admin", "/api/v1/admin/agents", "/api/v1/admin/other"} {
		app := authApp(kc, path)
		if got := statusFor(t, app, http.MethodPost, path, "", "good"); got != http.StatusUnauthorized {
			t.Errorf("%s accepted a Bearer (%d); only the toolkit's own paths may", path, got)
		}
	}
}

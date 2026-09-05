// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/api"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/middleware"
)

// The wiring half of moving the NOC portals onto cookies. The handlers are tested next to
// their own code; what can only be seen here is whether the CSRF guard actually covers the
// service, and whether the login stayed reachable to someone who has no session yet.

func authWiredApp(t *testing.T) *fiber.App {
	t.Helper()
	app := fiber.New()
	kc := fakeKeycloak{roles: map[string][]string{
		"noc-admin": {"ROLE_NOC_ADMIN"},
	}}
	mountRoutes(app, kc, nocRoutes{
		Push:   probeHandler{path: "/push"},
		Admin:  []routeRegistrar{probeHandler{path: "/spokes"}},
		Portal: []routeRegistrar{probeHandler{path: "/pools"}},
		// The real handler, not a probe: whether the login is reachable depends on where
		// it sits relative to the portal group's "/api/v1" prefix, and a stand-in would
		// not exercise that.
		Auth: api.NewAuthHandler(kc, false, []byte("test-secret")),
	}, []byte("test-secret"))
	return app
}

func send(t *testing.T, app *fiber.App, method, path string, cookies []*http.Cookie, headers map[string]string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

// TestLoginIsReachableWithoutASession is the one that would break the portal outright: if
// the auth group ends up behind RequireAuth, nobody can ever obtain the cookie that
// RequireAuth wants.
func TestLoginIsReachableWithoutASession(t *testing.T) {
	resp := send(t, authWiredApp(t), http.MethodPost, "/api/v1/auth/login", nil, nil)
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatal("POST /api/v1/auth/login answered 401 with no session — the login is behind the " +
			"very guard it exists to satisfy, and the portal can never log in")
	}
}

// TestCSRFCoversAMutatingRequestOnAnUnroutedPath proves the guard the way the api-gateways
// prove it: a path with no handler must still receive the CSRF refusal. That is what
// distinguishes "mounted app-wide" from "attached to the groups that existed when it was
// written" — the wiring defect that left 27 mutating routes unguarded in the v2 tree.
func TestCSRFCoversAMutatingRequestOnAnUnroutedPath(t *testing.T) {
	resp := send(t, authWiredApp(t), http.MethodPost, "/api/v1/no/such/route",
		[]*http.Cookie{{Name: middleware.SessionCookieName, Value: "noc-admin"}}, nil)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 — a mutating request with a session and no CSRF header "+
			"must be refused even where no route exists", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["code"] != middleware.CodeCSRFTokenInvalid {
		t.Errorf("code = %v, want %s — without the code this test would pass on any 403, "+
			"including a role refusal, and prove nothing about CSRF",
			body["code"], middleware.CodeCSRFTokenInvalid)
	}
}

// TestCSRFDoesNotBlockTheToolkitsBearerCall pins the other half of the Bearer decision. The
// toolkit sends no cookie, so there is no ambient credential for CSRF to defend and the
// guard must let it through — while a browser, which always sends its cookie, stays covered
// by the test above.
func TestCSRFDoesNotBlockTheToolkitsBearerCall(t *testing.T) {
	resp := send(t, authWiredApp(t), http.MethodPost, "/api/v1/admin/agents/provision-key", nil,
		map[string]string{"Authorization": "Bearer noc-admin"})

	if resp.StatusCode == http.StatusForbidden {
		t.Fatal("the toolkit's cookieless Bearer call was refused by CSRF; NOC agent provisioning " +
			"would fail on every deploy")
	}
}

// TestCSRFAllowsAMutatingRequestCarryingTheDoubleSubmit is the positive case: with a session
// cookie and a matching, session-bound header, the request proceeds.
func TestCSRFAllowsAMutatingRequestCarryingTheDoubleSubmit(t *testing.T) {
	token, err := middleware.NewCSRFToken([]byte("test-secret"), "noc-admin")
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	resp := send(t, authWiredApp(t), http.MethodPost, "/api/v1/admin/spokes",
		[]*http.Cookie{
			{Name: middleware.SessionCookieName, Value: "noc-admin"},
			{Name: middleware.CSRFCookieName, Value: token},
		},
		map[string]string{middleware.CSRFHeaderName: token})

	if resp.StatusCode == http.StatusForbidden {
		t.Fatalf("a correct double-submit was refused (403) — the guard rejects legitimate traffic")
	}
}

// TestGETIsNotCSRFGuarded keeps the read surface usable: a safe method carries no CSRF
// requirement, and OPTIONS matters as much as GET because it is the CORS preflight.
func TestGETIsNotCSRFGuarded(t *testing.T) {
	resp := send(t, authWiredApp(t), http.MethodGet, "/api/v1/pools",
		[]*http.Cookie{{Name: middleware.SessionCookieName, Value: "noc-admin"}}, nil)

	if resp.StatusCode == http.StatusForbidden {
		t.Error("a GET with a session was refused by CSRF; reads must not require the header")
	}
}

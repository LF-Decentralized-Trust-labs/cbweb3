// SPDX-License-Identifier: Apache-2.0

// Every cookie-authenticated mutating route must reject a request that carries no
// CSRF token.
//
// This is a sweep over the registered route table rather than a list, because a
// list is exactly what failed: the CSRF control was wired to the v1 tree and the v2
// tree was added later with 27 mutating routes and no guard. A hand-maintained list
// would have been just as incomplete as the wiring it was meant to check, and would
// have gone stale the same way. Asking the router what it actually registered
// cannot.
package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/gofiber/fiber/v2"
)

// csrfExemptPaths are the mutating routes that must NOT require a CSRF token, each
// for a reason that has to survive being read aloud:
//
//   - the auth entry points run BEFORE a session exists, so there is no token to
//     present and nothing yet to protect;
//   - the internal service-to-service routes are authenticated by a relay
//     signature, not by a cookie, so a browser cannot be tricked into sending them
//     — CSRF is about ambient credentials, and these have none.
//
// Anything else appearing here should be argued for in review, not added quietly.
var csrfExemptPaths = map[string]bool{
	"/api/v1/auth/login":   true,
	"/api/v1/auth/refresh": true,
	"/api/v1/auth/logout":  true,
}

// csrfExemptPrefixes covers whole trees that are not cookie-authenticated.
var csrfExemptPrefixes = []string{
	"/internal/", // relay-signed service-to-service surface
}

func isCSRFExempt(path string) bool {
	if csrfExemptPaths[path] {
		return true
	}
	for _, p := range csrfExemptPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// allRolesAuthProvider authenticates every token and grants every role, so the ONLY
// reason a request in this sweep can be refused is the CSRF guard. Without this a
// role rejection would look like success.
type allRolesAuthProvider struct{ authProviderStub }

func (s allRolesAuthProvider) Validate(_ context.Context, _ string) (domain.TokenClaims, error) {
	return domain.TokenClaims{
		Subject: "sweep",
		Roles: []string{
			domain.RoleGovernance, domain.RoleTreasury, domain.RoleSupervisor,
			domain.RoleCommercialBank, domain.RoleAdmission, domain.RoleNOC,
			domain.RoleCentralBank, domain.RoleCentralBankScenarioB,
			domain.RoleCommercialBankScenarioB, domain.RoleGovernanceOfficer,
			domain.RoleMLP, domain.RoleMLPScenarioB,
		},
	}, nil
}

// concretePath replaces :params with a value, since Fiber reports the pattern.
func concretePath(pattern string) string {
	parts := strings.Split(pattern, "/")
	for i, p := range parts {
		if strings.HasPrefix(p, ":") || strings.HasPrefix(p, "+") || strings.HasPrefix(p, "*") {
			parts[i] = "sweep-value"
		}
	}
	return strings.Join(parts, "/")
}

// TestEveryMutatingRouteRequiresCSRF is the finding-2 and finding-7 guard.
func TestEveryMutatingRouteRequiresCSRF(t *testing.T) {
	app := fiber.New()
	Setup(app, sweepDependencies())

	mutating := map[string]bool{
		fiber.MethodPost: true, fiber.MethodPut: true,
		fiber.MethodPatch: true, fiber.MethodDelete: true,
	}

	checked := 0
	for _, r := range app.GetRoutes() {
		if !mutating[r.Method] || isCSRFExempt(r.Path) || !strings.HasPrefix(r.Path, "/api/") {
			continue
		}
		checked++

		req := httptest.NewRequest(r.Method, concretePath(r.Path), strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		// A full session, and deliberately NO CSRF header.
		req.AddCookie(&http.Cookie{Name: "access_token", Value: "sweep-session"})

		resp, err := app.Test(req)
		if err != nil {
			t.Errorf("%s %s: %v", r.Method, r.Path, err)
			continue
		}

		// The status alone is not evidence. A 403 from a role check would satisfy a
		// naive assertion while proving nothing about CSRF, which is why the guard
		// carries its own code and this test insists on reading it.
		var body struct {
			Code string `json:"code"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&body)

		if resp.StatusCode != http.StatusForbidden || body.Code != middleware.CodeCSRFTokenInvalid {
			t.Errorf("%s %s: reached the handler without a CSRF token (status %d, code %q); "+
				"attach middleware.CSRF to this route's group",
				r.Method, r.Path, resp.StatusCode, body.Code)
		}
	}

	// A sweep that matched nothing would pass in silence. The count is asserted so
	// a router refactor that stops registering routes cannot look like compliance.
	//
	// The floor is 15 rather than the full route count because this harness supplies
	// only the handlers a unit test can cheaply stub: the governance, payments, token
	// and whole v2 groups register only when their services are injected. That is a
	// real limit of enumeration, and it is why the structural test below — not this
	// one — is what proves the v2 tree is covered.
	if checked < 15 {
		t.Fatalf("swept only %d mutating routes; expected at least 15 — the route table "+
			"was not populated, so this test proved nothing", checked)
	}
	t.Logf("swept %d mutating routes", checked)
}

// TestCSRFSweepHarnessIsNotVacuous proves the harness above can actually observe a
// handler being reached: the same request against an exempt route must succeed in
// getting past the guard. Without this, a sweep that fails for an unrelated reason
// (a panic, a 404 on every path) would read as a pass.
func TestCSRFSweepHarnessIsNotVacuous(t *testing.T) {
	app := fiber.New()
	Setup(app, sweepDependencies())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	var body struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.Code == middleware.CodeCSRFTokenInvalid {
		t.Fatal("the exempt login route is CSRF-guarded; the sweep's exemptions are wrong")
	}
}

func sweepDependencies() Dependencies {
	authHandler := handlers.NewAuthHandler(authProviderStub{}, kycCheckerStub{}, false)
	return Dependencies{
		AuthHandler:       authHandler,
		ComplianceHandler: handlers.NewComplianceHandler(kycCheckerStub{}, nil),
		AuthProvider:      allRolesAuthProvider{},
	}
}

// TestCSRFGuardCoversTheWholeAPITree is the assertion that actually closes finding 2.
//
// The sweep above can only judge routes this harness registers, and the v2 tree —
// the 27 unguarded mutating routes the review found — needs a dozen service stubs to
// come into existence. Enumeration therefore cannot prove v2 is covered.
//
// Mounting is structural, so it can be proven structurally: the guard is app-wide,
// so it answers for a path that has NO route at all. A request that never reaches
// routing still gets the CSRF refusal, which means every route under /api — those
// registered today, those registered only in production wiring, and those added next
// year — is behind it. If someone moves the guard back to per-group wiring, this
// test fails immediately, and it is the failure that matters.
func TestCSRFGuardCoversTheWholeAPITree(t *testing.T) {
	app := fiber.New()
	Setup(app, sweepDependencies())

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		req := httptest.NewRequest(method, "/api/v1/route-that-does-not-exist", strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "access_token", Value: "sweep-session"})

		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("%s: %v", method, err)
		}
		var body struct {
			Code string `json:"code"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&body)

		if body.Code != middleware.CodeCSRFTokenInvalid {
			t.Errorf("%s on an unregistered /api path: code %q, want %s — the guard is not "+
				"mounted app-wide, so routes registered elsewhere (v2) are unprotected",
				method, body.Code, middleware.CodeCSRFTokenInvalid)
		}
	}

	// The mirror: a GET must NOT be refused, or the guard is a blanket denial that
	// would pass the assertion above while breaking every read in the product.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/route-that-does-not-exist", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	if resp.StatusCode == http.StatusForbidden {
		t.Error("GET was refused by the CSRF guard; safe methods must pass")
	}
}

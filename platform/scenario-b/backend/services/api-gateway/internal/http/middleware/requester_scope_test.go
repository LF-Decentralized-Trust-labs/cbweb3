// SPDX-License-Identifier: Apache-2.0

package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/gofiber/fiber/v2"
)

// The payment listing handlers are shared between two route families: the central bank's own portal,
// where listing every record is the point, and /internal/v1/payments/*, which serves one commercial
// bank. The handler filters by requester_id and returns EVERYTHING when it is absent, so on the
// bank-facing family that parameter is not an optional refinement — it is the tenant boundary.
//
// Scoping it from the request was not enough. The signature covers method, path, body hash and
// timestamp — NOT the query string — so an onboarded bank can sign a listing request and hang another
// bank's address on it. That is why the central bank derives the value from the
// identity it verified instead of reading the parameter.

type stubScopeResolver struct {
	addr  string
	err   error
	asked string
}

func (s *stubScopeResolver) ResolveWalletAddress(_ context.Context, bankCode string) (string, error) {
	s.asked = bankCode
	return s.addr, s.err
}

// scopedApp echoes the requester_id the handler ends up seeing, which is the value that decides
// whose records are returned.
func scopedApp(t *testing.T, caller string, resolver middleware.RequesterScopeResolver) (*fiber.App, *string) {
	t.Helper()
	seen := new(string)
	app := fiber.New()
	handler := func(c *fiber.Ctx) error {
		*seen = c.Query("requester_id")
		return c.SendString("listed")
	}
	if caller == "" {
		app.Get("/internal/v1/payments/deposits", middleware.ScopeRequesterToCaller(resolver), handler)
	} else {
		app.Get("/internal/v1/payments/deposits",
			func(c *fiber.Ctx) error {
				c.Locals(middleware.RelayCallerLocal, caller)
				return c.Next()
			},
			middleware.ScopeRequesterToCaller(resolver), handler)
	}
	return app, seen
}

func TestScopeRequesterToCaller_OverwritesTheRequestedScope(t *testing.T) {
	resolver := &stubScopeResolver{addr: "0xITAU"}
	app, seen := scopedApp(t, "bank-itau", resolver)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet,
		"/internal/v1/payments/deposits?requester_id=0xVICTIM", nil), -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d; want 200 — a legitimate caller must still get its own listing", resp.StatusCode)
	}
	if *seen != "0xITAU" {
		t.Fatalf("handler saw requester_id=%q; want the caller's own address — the parameter must not decide the tenant", *seen)
	}
	if resolver.asked != "bank-itau" {
		t.Fatalf("resolved %q; want the verified caller's id", resolver.asked)
	}
}

func TestScopeRequesterToCaller_RefusesARequestWithNoVerifiedIdentity(t *testing.T) {
	app, seen := scopedApp(t, "", &stubScopeResolver{addr: "0xITAU"})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet,
		"/internal/v1/payments/deposits?requester_id=0xVICTIM", nil), -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d; want 401 — the shared secret names no caller to scope to", resp.StatusCode)
	}
	if *seen != "" {
		t.Fatal("the handler must not run at all")
	}
}

func TestScopeRequesterToCaller_RefusesACallerThatIsNotAParticipant(t *testing.T) {
	// The Cacti relay has a pinned identity but no participant record, and a deactivated bank has a
	// record that no longer resolves. Neither may be answered unscoped.
	app, seen := scopedApp(t, "cacti-relay", &stubScopeResolver{err: errors.New("participant not found")})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/internal/v1/payments/deposits", nil), -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d; want 403", resp.StatusCode)
	}
	if *seen != "" {
		t.Fatal("the handler must not run at all")
	}
}

func TestScopeRequesterToCaller_FailsClosedWithoutAResolver(t *testing.T) {
	app, seen := scopedApp(t, "bank-itau", nil)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet,
		"/internal/v1/payments/deposits?requester_id=0xITAU", nil), -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d; want 503 — an unscopeable listing must not be answered from the whole table", resp.StatusCode)
	}
	if *seen != "" {
		t.Fatal("the handler must not run at all")
	}
}

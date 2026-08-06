// SPDX-License-Identifier: Apache-2.0

package middleware_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/gofiber/fiber/v2"
)

// The listing routes were scoped to the verified caller; the CREATION routes on the same group were
// not. They read requester_besu_address straight from the body, and the body is written by the
// caller — so an onboarded bank, which holds a valid signing identity precisely so it can call
// /internal, could POST a deposit, escrow or redeem naming ANOTHER bank's address and have the
// central bank create the record against it. Once an operator approves it, the victim's fCeBM is
// burned or its tCeBM converted. The bank proxy injecting the address protects honest proxy traffic
// only, which is exactly what made the read-side leak a finding rather than a nit.
//
// So the write side derives the field the same way the read side derives the query parameter.

// bodyScopedApp echoes the body the handler ends up parsing, which is the value that decides whose
// records are created.
func bodyScopedApp(t *testing.T, caller string, resolver middleware.RequesterScopeResolver) (*fiber.App, *string) {
	t.Helper()
	seen := new(string)
	app := fiber.New()
	handler := func(c *fiber.Ctx) error {
		*seen = string(c.Body())
		return c.SendString("created")
	}
	app.Post("/internal/v1/payments/deposits",
		func(c *fiber.Ctx) error {
			if caller != "" {
				c.Locals(middleware.RelayCallerLocal, caller)
			}
			return c.Next()
		},
		middleware.ScopeRequesterBodyToCaller(resolver), handler)
	return app, seen
}

func postJSON(t *testing.T, app *fiber.App, path, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func TestScopeRequesterBodyToCaller_OverwritesTheNamedInstitution(t *testing.T) {
	resolver := &stubScopeResolver{addr: "0xITAU"}
	app, seen := bodyScopedApp(t, "bank-itau", resolver)

	resp := postJSON(t, app, "/internal/v1/payments/deposits",
		`{"requester_besu_address":"0xVICTIM","amount":"1000"}`)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d; want 200 — a legitimate caller must still be able to create its own record", resp.StatusCode)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(*seen), &got); err != nil {
		t.Fatalf("handler received a body it cannot parse (%q): %v", *seen, err)
	}
	if got["requester_besu_address"] != "0xITAU" {
		t.Fatalf("handler saw requester_besu_address=%q; want the caller's own address — the body must not decide the tenant",
			got["requester_besu_address"])
	}
	if got["amount"] != "1000" {
		t.Fatalf("amount = %q; want the rest of the body preserved verbatim", got["amount"])
	}
	if resolver.asked != "bank-itau" {
		t.Fatalf("resolved %q; want the verified caller's id", resolver.asked)
	}
}

func TestScopeRequesterBodyToCaller_InjectsTheAddressWhenTheBodyOmitsIt(t *testing.T) {
	// The honest proxy always sends it, so an absent field is not a normal request — but leaving it
	// absent would hand the handler an empty address and a 400 that says nothing about identity.
	app, seen := bodyScopedApp(t, "bank-itau", &stubScopeResolver{addr: "0xITAU"})

	resp := postJSON(t, app, "/internal/v1/payments/deposits", `{"amount":"1000"}`)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d; want 200", resp.StatusCode)
	}
	if !strings.Contains(*seen, `"requester_besu_address":"0xITAU"`) {
		t.Fatalf("handler saw %q; want the caller's address injected", *seen)
	}
}

func TestScopeRequesterBodyToCaller_RefusesARequestWithNoVerifiedIdentity(t *testing.T) {
	app, seen := bodyScopedApp(t, "", &stubScopeResolver{addr: "0xITAU"})

	resp := postJSON(t, app, "/internal/v1/payments/deposits",
		`{"requester_besu_address":"0xVICTIM","amount":"1000"}`)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d; want 401 — the shared secret names no caller to create records for", resp.StatusCode)
	}
	if *seen != "" {
		t.Fatal("the handler must not run at all")
	}
}

func TestScopeRequesterBodyToCaller_RefusesACallerThatIsNotAParticipant(t *testing.T) {
	app, seen := bodyScopedApp(t, "cacti-relay", &stubScopeResolver{err: errors.New("participant not found")})

	resp := postJSON(t, app, "/internal/v1/payments/deposits", `{"amount":"1000"}`)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d; want 403", resp.StatusCode)
	}
	if *seen != "" {
		t.Fatal("the handler must not run at all")
	}
}

func TestScopeRequesterBodyToCaller_FailsClosedWithoutAResolver(t *testing.T) {
	app, seen := bodyScopedApp(t, "bank-itau", nil)

	resp := postJSON(t, app, "/internal/v1/payments/deposits",
		`{"requester_besu_address":"0xITAU","amount":"1000"}`)

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d; want 503 — an unbindable creation must not be written from the body", resp.StatusCode)
	}
	if *seen != "" {
		t.Fatal("the handler must not run at all")
	}
}

func TestScopeRequesterBodyToCaller_RefusesABodyItCannotBind(t *testing.T) {
	// Not a JSON object: the field cannot be replaced, so the handler must not see the original
	// bytes either — that is the state the whole middleware exists to prevent.
	app, seen := bodyScopedApp(t, "bank-itau", &stubScopeResolver{addr: "0xITAU"})

	resp := postJSON(t, app, "/internal/v1/payments/deposits", `["not","an","object"]`)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d; want 400", resp.StatusCode)
	}
	if *seen != "" {
		t.Fatal("the handler must not run at all")
	}
}

// --- deposit binding (POST /internal/v1/payments/deposits/exchange) ---

type stubDepositOwnership struct {
	owns     bool
	err      error
	askedFor string
	askedID  string
}

func (s *stubDepositOwnership) OwnsDeposit(_ context.Context, requesterAddress, depositID string) (bool, error) {
	s.askedFor = requesterAddress
	s.askedID = depositID
	return s.owns, s.err
}

func depositBoundApp(t *testing.T, caller string, resolver middleware.RequesterScopeResolver,
	owner middleware.DepositOwnership) (*fiber.App, *bool) {
	t.Helper()
	ran := new(bool)
	app := fiber.New()
	app.Post("/internal/v1/payments/deposits/exchange",
		func(c *fiber.Ctx) error {
			if caller != "" {
				c.Locals(middleware.RelayCallerLocal, caller)
			}
			return c.Next()
		},
		middleware.BindDepositToCaller(resolver, owner),
		func(c *fiber.Ctx) error {
			*ran = true
			return c.SendString("minted")
		})
	return app, ran
}

func TestBindDepositToCaller_RefusesAnotherBanksDeposit(t *testing.T) {
	// The exchange route carries no address to overwrite — it names a deposit id, and the mint that
	// follows credits whoever registered that deposit. Unbound, one bank can drive the fCeBM mint on
	// another bank's approved deposit: not a diversion of value, but an act on the victim's record
	// that its operator never asked for.
	owner := &stubDepositOwnership{owns: false}
	app, ran := depositBoundApp(t, "bank-itau", &stubScopeResolver{addr: "0xITAU"}, owner)

	resp := postJSON(t, app, "/internal/v1/payments/deposits/exchange", `{"deposit_id":"dep-of-victim"}`)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d; want 403", resp.StatusCode)
	}
	if *ran {
		t.Fatal("the handler must not run at all")
	}
	if owner.askedFor != "0xITAU" || owner.askedID != "dep-of-victim" {
		t.Fatalf("ownership asked for (%q, %q); want the verified caller's address and the named deposit",
			owner.askedFor, owner.askedID)
	}
}

func TestBindDepositToCaller_AllowsTheCallersOwnDeposit(t *testing.T) {
	app, ran := depositBoundApp(t, "bank-itau", &stubScopeResolver{addr: "0xITAU"},
		&stubDepositOwnership{owns: true})

	resp := postJSON(t, app, "/internal/v1/payments/deposits/exchange", `{"deposit_id":"dep-1"}`)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d; want 200", resp.StatusCode)
	}
	if !*ran {
		t.Fatal("the handler must run for the deposit's own owner")
	}
}

func TestBindDepositToCaller_FailsClosedWhenOwnershipCannotBeRead(t *testing.T) {
	app, ran := depositBoundApp(t, "bank-itau", &stubScopeResolver{addr: "0xITAU"},
		&stubDepositOwnership{err: errors.New("orchestrator unavailable")})

	resp := postJSON(t, app, "/internal/v1/payments/deposits/exchange", `{"deposit_id":"dep-1"}`)

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d; want 503 — an unverifiable owner must not be assumed to be the caller", resp.StatusCode)
	}
	if *ran {
		t.Fatal("the handler must not run at all")
	}
}

func TestBindDepositToCaller_LeavesAMissingDepositIDToTheHandler(t *testing.T) {
	// Refusing here too would answer 403 to what is really a malformed request, and the handler's
	// 400 is the message an operator can act on.
	app, ran := depositBoundApp(t, "bank-itau", &stubScopeResolver{addr: "0xITAU"},
		&stubDepositOwnership{owns: false})

	resp := postJSON(t, app, "/internal/v1/payments/deposits/exchange", `{}`)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d; want the handler to answer", resp.StatusCode)
	}
	if !*ran {
		t.Fatal("the handler must run so it can report the missing deposit_id")
	}
}

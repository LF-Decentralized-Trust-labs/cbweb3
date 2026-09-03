// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

const testSession = "session-abc"

var testSecret = []byte("test-secret-not-a-real-key")

// csrfApp builds a minimal app whose only protection is the guard under test, so a
// rejection can only have come from CSRF.
func csrfApp(t *testing.T) *fiber.App {
	t.Helper()
	app := fiber.New()
	app.Use(CSRF(CSRFConfig{
		Secret:    testSecret,
		SessionID: func(c *fiber.Ctx) string { return c.Cookies("access_token") },
	}))
	handler := func(c *fiber.Ctx) error { return c.SendString("ok") }
	app.Get("/r", handler)
	app.Post("/r", handler)
	app.Put("/r", handler)
	app.Patch("/r", handler)
	app.Delete("/r", handler)
	app.Options("/r", handler)
	return app
}

func csrfRequest(t *testing.T, app *fiber.App, method, header, cookie, session string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(method, "/r", strings.NewReader("{}"))
	if session != "" {
		req.AddCookie(&http.Cookie{Name: "access_token", Value: session})
	}
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: cookie})
	}
	if header != "" {
		req.Header.Set(CSRFHeaderName, header)
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	return resp
}

// TestCSRF_SafeMethodsPass covers the methods that must never be gated. OPTIONS is
// the one that would break everything if it were: it is the CORS preflight, and
// rejecting it fails every cross-origin request before the real one is sent.
func TestCSRF_SafeMethodsPass(t *testing.T) {
	app := csrfApp(t)
	for _, m := range []string{http.MethodGet, http.MethodOptions} {
		resp := csrfRequest(t, app, m, "", "", testSession)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s with no CSRF token: got %d, want 200", m, resp.StatusCode)
		}
	}
}

// TestCSRF_MutatingMethodsRequireTheToken is the guard's reason to exist.
func TestCSRF_MutatingMethodsRequireTheToken(t *testing.T) {
	app := csrfApp(t)
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		resp := csrfRequest(t, app, m, "", "", testSession)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s with no CSRF token: got %d, want 403", m, resp.StatusCode)
		}
	}
}

// TestCSRF_ValidTokenPasses is the other half: the guard must not be a blanket
// refusal. A check that rejects everything passes the test above and breaks the
// product.
func TestCSRF_ValidTokenPasses(t *testing.T) {
	token, err := NewCSRFToken(testSecret, testSession)
	if err != nil {
		t.Fatalf("NewCSRFToken: %v", err)
	}
	resp := csrfRequest(t, csrfApp(t), http.MethodPost, token, token, testSession)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("POST with a matching, session-bound token: got %d, want 200", resp.StatusCode)
	}
}

// TestCSRF_EmptyHeaderIsNotAMatch pins the classic double-submit bug: with both
// halves absent, a naive equality check finds "" == "" and lets the request through.
func TestCSRF_EmptyHeaderIsNotAMatch(t *testing.T) {
	resp := csrfRequest(t, csrfApp(t), http.MethodPost, "", "", testSession)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("POST with empty header AND empty cookie: got %d, want 403 — "+
			"an empty pair must never compare equal", resp.StatusCode)
	}
}

// TestCSRF_MismatchedHalvesRejected covers an attacker who can set a header but not
// read the cookie, which is the ordinary cross-site case.
func TestCSRF_MismatchedHalvesRejected(t *testing.T) {
	token, _ := NewCSRFToken(testSecret, testSession)
	other, _ := NewCSRFToken(testSecret, testSession)
	resp := csrfRequest(t, csrfApp(t), http.MethodPost, other, token, testSession)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("POST with mismatched halves: got %d, want 403", resp.StatusCode)
	}
}

// TestCSRF_TokenIsBoundToItsSession is the finding-4 property. Without the HMAC
// binding, any cookie-write primitive — a sibling subdomain, a MITM on plain HTTP —
// lets an attacker supply both halves of a token the server never issued.
func TestCSRF_TokenIsBoundToItsSession(t *testing.T) {
	token, _ := NewCSRFToken(testSecret, "session-one")

	// Same token, replayed against a different session: both halves match each
	// other, so only the binding can reject it.
	resp := csrfRequest(t, csrfApp(t), http.MethodPost, token, token, "session-two")
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("a token minted for another session was accepted: got %d, want 403", resp.StatusCode)
	}

	// An attacker-chosen value present in both halves must fail too: it carries no
	// MAC this server could have produced.
	forged := "attacker-chosen.deadbeef"
	resp = csrfRequest(t, csrfApp(t), http.MethodPost, forged, forged, testSession)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("a forged token present in both halves was accepted: got %d, want 403", resp.StatusCode)
	}
}

// TestCSRF_NoSessionIsRefused: with no session there is nothing to bind to, and the
// absence must not act as a wildcard that skips the check.
func TestCSRF_NoSessionIsRefused(t *testing.T) {
	token, _ := NewCSRFToken(testSecret, testSession)
	resp := csrfRequest(t, csrfApp(t), http.MethodPost, token, token, "")
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("POST with no session: got %d, want 403", resp.StatusCode)
	}
}

// TestCSRF_RefusalCarriesItsOwnCode is what makes the router-level test honest.
// Both a CSRF refusal and an authorization refusal are 403; without a distinct code
// a sweep asserting "403 on a missing header" would pass on any route that rejected
// the caller's role instead, proving nothing about CSRF.
func TestCSRF_RefusalCarriesItsOwnCode(t *testing.T) {
	resp := csrfRequest(t, csrfApp(t), http.MethodPost, "", "", testSession)
	body := make([]byte, 256)
	n, _ := resp.Body.Read(body)
	if !strings.Contains(string(body[:n]), CodeCSRFTokenInvalid) {
		t.Errorf("refusal body does not carry %s: %s", CodeCSRFTokenInvalid, body[:n])
	}
}

// TestNewCSRFToken_IsUnpredictable: two tokens for the same session must differ, or
// the value is guessable from a single observation.
func TestNewCSRFToken_IsUnpredictable(t *testing.T) {
	a, _ := NewCSRFToken(testSecret, testSession)
	b, _ := NewCSRFToken(testSecret, testSession)
	if a == b {
		t.Error("two tokens minted for the same session are identical")
	}
}

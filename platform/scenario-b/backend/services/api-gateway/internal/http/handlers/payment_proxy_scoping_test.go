// SPDX-License-Identifier: Apache-2.0

package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/relayauth"
	"github.com/gofiber/fiber/v2"
)

// The central bank scopes these listings by the requester_id query parameter, and lists EVERY record
// when it is absent. The write paths inject requester_besu_address from this gateway's own identity,
// but the read paths forwarded the caller's query string verbatim — and the portal sends no
// requester_id. Two consequences, observed live: every bank saw every other bank's issuance requests,
// and a bank could also name another bank's address on purpose and read its records.
//
// The identity of the calling institution belongs to the gateway, never to its caller. These tests
// pin that: the parameter is set from the entity address and a caller-supplied one is overridden.

// listedRequesterID drives one proxied listing and reports the requester_id the central bank received.
func listedRequesterID(t *testing.T, route string, register func(*handlers.PaymentProxyHandler, *fiber.App), requestURL string) (string, url.Values) {
	t.Helper()

	var got url.Values
	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"deposits":[]}`))
	}))
	defer cb.Close()

	h := handlers.NewPaymentProxyHandler(cb.URL, "0xITAU", "shh")
	app := fiber.New()
	register(h, app)

	if _, err := app.Test(httptest.NewRequest(http.MethodGet, requestURL, nil), -1); err != nil {
		t.Fatalf("proxy %s: %v", route, err)
	}
	return got.Get("requester_id"), got
}

func TestPaymentProxy_ScopesListingsToThisBank(t *testing.T) {
	cases := []struct {
		name     string
		register func(*handlers.PaymentProxyHandler, *fiber.App)
		path     string
	}{
		{"deposits", func(h *handlers.PaymentProxyHandler, a *fiber.App) { a.Get("/payments/deposits", h.ListDeposits) }, "/payments/deposits"},
		{"escrows", func(h *handlers.PaymentProxyHandler, a *fiber.App) { a.Get("/payments/escrows", h.ListEscrows) }, "/payments/escrows"},
		{"redeems", func(h *handlers.PaymentProxyHandler, a *fiber.App) { a.Get("/payments/redeems", h.ListRedeems) }, "/payments/redeems"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := listedRequesterID(t, tc.name, tc.register, tc.path)
			if got != "0xITAU" {
				t.Fatalf("requester_id = %q; want %q — an absent scope makes the central bank list every bank's records", got, "0xITAU")
			}
		})
	}
}

func TestPaymentProxy_OverridesACallerSuppliedRequesterID(t *testing.T) {
	cases := []struct {
		name     string
		register func(*handlers.PaymentProxyHandler, *fiber.App)
		path     string
	}{
		{"deposits", func(h *handlers.PaymentProxyHandler, a *fiber.App) { a.Get("/payments/deposits", h.ListDeposits) }, "/payments/deposits?requester_id=0xBRADESCO"},
		{"escrows", func(h *handlers.PaymentProxyHandler, a *fiber.App) { a.Get("/payments/escrows", h.ListEscrows) }, "/payments/escrows?requester_id=0xBRADESCO"},
		{"redeems", func(h *handlers.PaymentProxyHandler, a *fiber.App) { a.Get("/payments/redeems", h.ListRedeems) }, "/payments/redeems?requester_id=0xBRADESCO"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, all := listedRequesterID(t, tc.name, tc.register, tc.path)
			if got != "0xITAU" {
				t.Fatalf("requester_id = %q; want %q — a caller must not choose whose records it reads", got, "0xITAU")
			}
			if len(all["requester_id"]) != 1 {
				t.Fatalf("requester_id appears %d times (%v); a repeated parameter lets the receiver pick the attacker's copy",
					len(all["requester_id"]), all["requester_id"])
			}
		})
	}
}

// Scoping must not eat the rest of the query string.
func TestPaymentProxy_PreservesOtherQueryParameters(t *testing.T) {
	_, all := listedRequesterID(t, "deposits",
		func(h *handlers.PaymentProxyHandler, a *fiber.App) { a.Get("/payments/deposits", h.ListDeposits) },
		"/payments/deposits?status=PENDING&limit=25")

	if all.Get("status") != "PENDING" || all.Get("limit") != "25" {
		t.Fatalf("other parameters were lost: %v", all)
	}
}

// The central bank builds its canonical string from the request PATH; a query string is not part of
// it. The proxy passes one string for both the URL it fetches and the value it signs, so appending the
// scope to that string signs "/internal/v1/payments/deposits?requester_id=..." while the receiver
// verifies "/internal/v1/payments/deposits" — every listing then fails with
// RELAY_SIGNATURE_INVALID. This was already reachable before scoping: any query parameter the portal
// sent had the same effect.
func TestPaymentProxy_SignatureVerifiesOverThePathWithoutTheQueryString(t *testing.T) {
	signer := newProxySigner(t, "bank-itau")
	reg := relayauth.NewRegistry()
	reg.Add("bank-itau", signer.PublicKey())

	var verifyErr error
	var sawScope string
	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Exactly what the central bank's middleware does: verify over r.URL.Path.
		verifyErr = reg.VerifyRequest(
			r.Header.Get(relayauth.HeaderKeyID),
			r.Header.Get(relayauth.HeaderTimestamp),
			r.Header.Get(relayauth.HeaderSignature),
			r.Method, r.URL.Path, nil, time.Now())
		sawScope = r.URL.Query().Get("requester_id")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"deposits":[]}`))
	}))
	defer cb.Close()

	h := handlers.NewPaymentProxyHandler(cb.URL, "0xITAU", "shh").WithSigner(signer)
	app := fiber.New()
	app.Get("/payments/deposits", h.ListDeposits)

	if _, err := app.Test(httptest.NewRequest(http.MethodGet, "/payments/deposits", nil), -1); err != nil {
		t.Fatalf("proxy: %v", err)
	}
	if sawScope != "0xITAU" {
		t.Fatalf("the scope did not reach the central bank: %q", sawScope)
	}
	if verifyErr != nil {
		t.Fatalf("a scoped listing did not verify at the receiver: %v", verifyErr)
	}
}

// An entity with no address configured must not silently fall back to listing everything.
func TestPaymentProxy_RefusesToListWithoutAnEntityAddress(t *testing.T) {
	reached := false
	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))
	defer cb.Close()

	h := handlers.NewPaymentProxyHandler(cb.URL, "", "shh")
	app := fiber.New()
	app.Get("/payments/deposits", h.ListDeposits)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/payments/deposits", nil), -1)
	if err != nil {
		t.Fatalf("proxy: %v", err)
	}
	if reached {
		t.Fatal("an unscoped listing reached the central bank; it would have returned every bank's records")
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d; want 500 with an explicit reason", resp.StatusCode)
	}
}

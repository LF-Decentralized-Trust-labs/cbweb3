// SPDX-License-Identifier: Apache-2.0

// This file covers the PaymentProxyHandler Fetch* helpers, which the statement
// handler uses to consolidate a bank's movements from the Central Bank's
// internal API. The CB is faked with httptest.Server; these helpers perform a
// relay-authenticated GET and decode JSON, so no payment gRPC backend is needed.
package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestPaymentProxy_Fetch verifies each Fetch* helper scopes the CB request to the
// entity (requester_id / bank_id), forwards the relay-auth header, and decodes the
// respective envelope.
func TestPaymentProxy_Fetch(t *testing.T) {
	t.Parallel()

	const entityBesuAddr = "0xbesu"
	const bankID = "bank-b"
	const relaySecret = "relay-secret"

	type call struct{ path, query, auth string }
	var calls []call

	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, call{r.URL.Path, r.URL.RawQuery, r.Header.Get("X-Relay-Auth")})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		switch r.URL.Path {
		case "/internal/v1/payments/deposits":
			_, _ = w.Write([]byte(`{"deposits":[{"id":"d1","amount":"10"}]}`))
		case "/internal/v1/payments/escrows":
			_, _ = w.Write([]byte(`{"escrows":[{"id":"e1","amount":"20"}]}`))
		case "/internal/v1/payments/redeems":
			_, _ = w.Write([]byte(`{"redeems":[{"id":"r1","amount":"30"}]}`))
		case "/internal/v1/payments/pvp-credits":
			_, _ = w.Write([]byte(`{"credits":[{"reference":"t1","amount":"40","settled_at":"2026-07-16T00:00:00Z"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(cb.Close)

	h := NewPaymentProxyHandler(cb.URL, nil, entityBesuAddr, "pal", "cbpal", relaySecret)
	ctx := context.Background()

	deposits, err := h.FetchDeposits(ctx)
	if err != nil || len(deposits) != 1 || deposits[0].ID != "d1" {
		t.Fatalf("FetchDeposits = %+v, err %v", deposits, err)
	}
	escrows, err := h.FetchEscrows(ctx)
	if err != nil || len(escrows) != 1 || escrows[0].ID != "e1" {
		t.Fatalf("FetchEscrows = %+v, err %v", escrows, err)
	}
	redeems, err := h.FetchRedeems(ctx)
	if err != nil || len(redeems) != 1 || redeems[0].ID != "r1" {
		t.Fatalf("FetchRedeems = %+v, err %v", redeems, err)
	}
	credits, err := h.FetchPvPCredits(ctx, bankID)
	if err != nil || len(credits) != 1 || credits[0].Reference != "t1" {
		t.Fatalf("FetchPvPCredits = %+v, err %v", credits, err)
	}

	// Every call must be relay-authenticated and scoped to this entity.
	wantScope := map[string]string{
		"/internal/v1/payments/deposits":    "requester_id=" + entityBesuAddr,
		"/internal/v1/payments/escrows":     "requester_id=" + entityBesuAddr,
		"/internal/v1/payments/redeems":     "requester_id=" + entityBesuAddr,
		"/internal/v1/payments/pvp-credits": "bank_id=" + bankID,
	}
	for _, c := range calls {
		if c.auth != relaySecret {
			t.Errorf("%s: missing/incorrect X-Relay-Auth %q", c.path, c.auth)
		}
		if want := wantScope[c.path]; c.query != want {
			t.Errorf("%s: query %q, want %q", c.path, c.query, want)
		}
	}
}

// TestPaymentProxy_Fetch_CBError asserts a non-200 from the Central Bank surfaces
// as an error rather than an empty result.
func TestPaymentProxy_Fetch_CBError(t *testing.T) {
	t.Parallel()

	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	t.Cleanup(cb.Close)

	h := NewPaymentProxyHandler(cb.URL, nil, "0xbesu", "pal", "cbpal", "")
	if _, err := h.FetchDeposits(context.Background()); err == nil {
		t.Error("FetchDeposits must return an error on CB 500")
	}
}

// TestPaymentProxy_Fetch_Unreachable asserts a transport error is propagated.
func TestPaymentProxy_Fetch_Unreachable(t *testing.T) {
	t.Parallel()
	h := NewPaymentProxyHandler("http://127.0.0.1:1", nil, "0xbesu", "pal", "cbpal", "")
	if _, err := h.FetchPvPCredits(context.Background(), "bank-b"); err == nil {
		t.Error("FetchPvPCredits must return an error when CB is unreachable")
	}
}

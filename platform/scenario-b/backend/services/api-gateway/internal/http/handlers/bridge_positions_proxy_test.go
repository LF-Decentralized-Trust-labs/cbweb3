// SPDX-License-Identifier: Apache-2.0

package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
)

// A commercial bank's own gateway holds no bridge positions: an incoming cross-currency
// delivery is recorded on its CENTRAL BANK's gateway. So the bank's portal asked its own
// gateway and got nothing, and the receiving institution saw a balance change with no record
// of where the money came from.
//
// These pin the hop that fixes it. The bank does not filter the listing — the central bank
// scopes it to the verified caller — so what matters here is that the request reaches the
// right path and that the answer is passed through unchanged.

func TestBridgePositionsProxy_AsksTheCentralBank(t *testing.T) {
	var gotPath, gotMethod string
	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"positions":[{"position_id":"pos-1","owner_bank_id":"bank-macro","bridge_state":"RELEASED"}]}`))
	}))
	defer cb.Close()

	h := handlers.NewPaymentProxyHandler(cb.URL, "0xMACRO", "shh")
	app := fiber.New()
	app.Get("/api/v2/bridge/positions", h.ListBridgePositions)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v2/bridge/positions", nil), -1)
	if err != nil {
		t.Fatalf("proxy: %v", err)
	}
	if gotMethod != http.MethodGet || gotPath != "/internal/v1/bridge/positions" {
		t.Errorf("central bank saw %s %s, want GET /internal/v1/bridge/positions", gotMethod, gotPath)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body struct {
		Positions []map[string]any `json:"positions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(body.Positions) != 1 || body.Positions[0]["position_id"] != "pos-1" {
		t.Errorf("the central bank's answer was not passed through: %+v", body.Positions)
	}
}

// TestBridgePositionsProxy_ForwardsTheStateFilter keeps the existing query contract: the
// portal may ask for one lifecycle state, and dropping it here would silently return
// everything.
func TestBridgePositionsProxy_ForwardsTheStateFilter(t *testing.T) {
	var gotState string
	cb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotState = r.URL.Query().Get("state")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"positions":[]}`))
	}))
	defer cb.Close()

	h := handlers.NewPaymentProxyHandler(cb.URL, "0xMACRO", "shh")
	app := fiber.New()
	app.Get("/api/v2/bridge/positions", h.ListBridgePositions)

	if _, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v2/bridge/positions?state=RELEASED", nil), -1); err != nil {
		t.Fatalf("proxy: %v", err)
	}
	if gotState != "RELEASED" {
		t.Errorf("state forwarded as %q, want RELEASED", gotState)
	}
}

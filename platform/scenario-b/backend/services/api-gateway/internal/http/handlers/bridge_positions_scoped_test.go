// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/middleware"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
)

// The internal listing a commercial bank reaches to see the payments it received. Its whole
// job is to answer for the VERIFIED caller and nobody else: the rows live on the central
// bank's gateway, where every bank's positions sit side by side.

type scopedPosReaderStub struct {
	gotOwner string
	gotState string
	err      error
}

func (s *scopedPosReaderStub) ListPositions(context.Context, string) ([]services.BridgePositionResult, error) {
	return nil, nil
}

func (s *scopedPosReaderStub) ListPositionsForOwner(_ context.Context, owner, state string) ([]services.BridgePositionResult, error) {
	s.gotOwner = owner
	s.gotState = state
	if s.err != nil {
		return nil, s.err
	}
	return []services.BridgePositionResult{{PositionID: "pos-1", OwnerBankID: owner, BridgeState: "RELEASED"}}, nil
}

// withVerifiedCaller stands in for the relay-auth middleware, which is what puts a verified
// identity on the request.
func withVerifiedCaller(caller string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if caller != "" {
			c.Locals(middleware.RelayCallerLocal, caller)
		}
		return c.Next()
	}
}

func TestListPositionsForCaller_ScopesToTheVerifiedCaller(t *testing.T) {
	t.Parallel()
	reader := &scopedPosReaderStub{}
	h := NewBridgeHandler(nil, nil, reader)

	app := fiber.New()
	app.Get("/internal/v1/bridge/positions", withVerifiedCaller("bank-macro"), h.ListPositionsForCaller)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/internal/v1/bridge/positions?state=RELEASED", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if reader.gotOwner != "bank-macro" {
		t.Errorf("listed for owner %q, want the verified caller bank-macro", reader.gotOwner)
	}
	if reader.gotState != "RELEASED" {
		t.Errorf("state filter = %q, want RELEASED", reader.gotState)
	}
}

// TestListPositionsForCaller_IgnoresAnOwnerFromTheQuery is the tenant boundary. A caller that
// names another bank must get its own rows, never that bank's — the same rule the payment
// listings enforce, and for the same reason: no signature covers a query parameter's meaning.
func TestListPositionsForCaller_IgnoresAnOwnerFromTheQuery(t *testing.T) {
	t.Parallel()
	reader := &scopedPosReaderStub{}
	h := NewBridgeHandler(nil, nil, reader)

	app := fiber.New()
	app.Get("/internal/v1/bridge/positions", withVerifiedCaller("bank-macro"), h.ListPositionsForCaller)

	_, err := app.Test(httptest.NewRequest(http.MethodGet,
		"/internal/v1/bridge/positions?owner_bank_id=bank-galicia", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if reader.gotOwner != "bank-macro" {
		t.Errorf("listed for owner %q; a caller naming another bank must still get its own rows",
			reader.gotOwner)
	}
}

// TestListPositionsForCaller_RefusesWithoutAVerifiedCaller fails closed. Without an identity
// there is no tenant, and answering would return the whole book.
func TestListPositionsForCaller_RefusesWithoutAVerifiedCaller(t *testing.T) {
	t.Parallel()
	reader := &scopedPosReaderStub{}
	h := NewBridgeHandler(nil, nil, reader)

	app := fiber.New()
	app.Get("/internal/v1/bridge/positions", withVerifiedCaller(""), h.ListPositionsForCaller)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/internal/v1/bridge/positions", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 — an unidentified caller has no tenant", resp.StatusCode)
	}
	if reader.gotOwner != "" {
		t.Errorf("the reader was queried for %q despite no verified caller", reader.gotOwner)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["positions"] != nil {
		t.Error("a refusal must not carry positions")
	}
	// The CODE, not just the status. The bank portal classifies a 401 by its code: a trust
	// rejection becomes a notice, and anything unclassified is taken for an expired session —
	// refresh, retry, fail again, log the operator out. This endpoint is called by the page
	// login lands on, so a code-less refusal ejected every bank the central bank cannot yet
	// verify, and the portal is the only intended route to onboarding. Asserting the status
	// alone is what let that ship.
	if got := body["code"]; got != "RELAY_CALLER_IDENTITY_REQUIRED" {
		t.Errorf("code = %v, want RELAY_CALLER_IDENTITY_REQUIRED — an unclassified 401 logs the operator out instead of explaining itself", got)
	}
}

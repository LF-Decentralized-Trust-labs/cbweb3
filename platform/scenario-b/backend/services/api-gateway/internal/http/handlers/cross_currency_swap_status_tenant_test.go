// SPDX-License-Identifier: Apache-2.0

// Tenant-scoping tests for GET /api/v2/amm/swap/cross-currency/:id (finding R2-M-10).
//
// The list endpoint on this handler already scopes to the caller (ListByPayer with
// claims.BankID), but the by-id read resolved a swap from the id alone. A swap id is a
// bearer of no authority: any authenticated commercial bank that holds or guesses one
// could read another bank's amounts, effective rate, counterparty and bridge position
// ids. These tests pin the read to the caller's own identity.
package handlers_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockOwnedSwapOrchestrator returns one swap owned by ownerBankID, regardless of the id
// asked for — the repository lookup is by id, which is exactly the surface under test.
type mockOwnedSwapOrchestrator struct {
	ownerBankID string
	// absent makes GetStatus report the swap as missing, the way the repository does for an
	// id that is not in the table. Needed to compare that answer with a cross-tenant denial.
	absent bool
	calls  int
}

func (m *mockOwnedSwapOrchestrator) Execute(_ context.Context, req services.CrossCurrencySwapRequest) (*services.CrossCurrencySwapResult, error) {
	return &services.CrossCurrencySwapResult{SwapID: req.SwapID, PayerBankID: m.ownerBankID, CreatedAt: time.Now()}, nil
}

func (m *mockOwnedSwapOrchestrator) GetStatus(_ context.Context, swapID string) (*services.CrossCurrencySwapResult, error) {
	m.calls++
	if m.absent {
		return nil, fmt.Errorf("swap %s not found: record not found", swapID)
	}
	return &services.CrossCurrencySwapResult{
		SwapID:        swapID,
		CorrelationID: "corr-1",
		PayerBankID:   m.ownerBankID,
		Status:        domain.SwapStatusCompleted,
		AmountIn:      "1234567",
		AmountOut:     "999999",
		EffectiveRate: 1.23,
		CreatedAt:     time.Now(),
	}, nil
}

// statusApp mounts only the by-id read, authenticated as callerBankID. An empty
// callerBankID injects claims with no bank; a nil claims marker omits them entirely.
func statusApp(orch handlers.CrossCurrencySwapOrchestratorIface, callerBankID string, withClaims bool, fallbackBankCode string) *fiber.App {
	h := handlers.NewCrossCurrencySwapHandler(orch, fallbackBankCode)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if withClaims {
			c.Locals("claims", domain.TokenClaims{Subject: "user-1", BankID: callerBankID})
		}
		return c.Next()
	})
	app.Get("/swap/:id", h.GetSwapStatus)
	return app
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(b)
}

// A bank must not read a swap that belongs to another bank. 404 rather than 403: a 403
// would confirm the id exists, which is itself a disclosure on an enumerable surface.
func TestGetSwapStatus_ForeignSwapIsNotReadable(t *testing.T) {
	orch := &mockOwnedSwapOrchestrator{ownerBankID: "bank-a"}
	app := statusApp(orch, "bank-b", true, "")

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/swap/SWAP-001", nil), -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	body := readBody(t, resp)
	assert.NotContains(t, body, "1234567", "the foreign swap's amount_in must not leak")
	assert.NotContains(t, body, "999999", "the foreign swap's amount_out must not leak")
	assert.NotContains(t, body, "corr-1", "the foreign swap's correlation id must not leak")
}

// The owner still reads its own swap in full.
func TestGetSwapStatus_OwnSwapIsReadable(t *testing.T) {
	orch := &mockOwnedSwapOrchestrator{ownerBankID: "bank-a"}
	app := statusApp(orch, "bank-a", true, "")

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/swap/SWAP-001", nil), -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, readBody(t, resp), "1234567")
}

// No authenticated claims → 401, and the orchestrator is never consulted.
func TestGetSwapStatus_Unauthenticated(t *testing.T) {
	orch := &mockOwnedSwapOrchestrator{ownerBankID: "bank-a"}
	app := statusApp(orch, "", false, "")

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/swap/SWAP-001", nil), -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Zero(t, orch.calls, "an unauthenticated read must not reach the orchestrator")
}

// Claims without a bank fall back to the gateway's own bank code, mirroring ListSwaps.
// A gateway deployed for bank-a therefore reads bank-a's swap.
func TestGetSwapStatus_FallsBackToGatewayBankCode(t *testing.T) {
	orch := &mockOwnedSwapOrchestrator{ownerBankID: "bank-a"}
	app := statusApp(orch, "", true, "bank-a")

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/swap/SWAP-001", nil), -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// Neither a bank in the claims nor a configured fallback → 401, never an unscoped read.
func TestGetSwapStatus_NoResolvableBankIsRejected(t *testing.T) {
	orch := &mockOwnedSwapOrchestrator{ownerBankID: "bank-a"}
	app := statusApp(orch, "", true, "")

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/swap/SWAP-001", nil), -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Zero(t, orch.calls, "a read with no resolvable bank must not reach the orchestrator")
}

// The owner comparison must not hinge on incidental case or padding differences between
// the claim and the stored owner.
func TestGetSwapStatus_OwnerComparisonIgnoresCaseAndPadding(t *testing.T) {
	orch := &mockOwnedSwapOrchestrator{ownerBankID: "Bank-A"}
	app := statusApp(orch, strings.ToLower(" bank-a "), true, "")

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/swap/SWAP-001", nil), -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// The load-bearing property of answering 404 instead of 403: a swap owned by another bank
// must be indistinguishable from a swap that does not exist. If the two answers differ in
// any byte — an extra field, a different error_code, a different status — the endpoint
// becomes an existence oracle and the disclosure this finding closes is back, because a
// caller can still confirm which ids are real.
//
// Both branches go through respondSwapNotFound for that reason; this test is what stops a
// later edit from splitting them apart again with the rest of the suite still green.
func TestGetSwapStatus_ForeignAndAbsentAreByteIdentical(t *testing.T) {
	foreign := &mockOwnedSwapOrchestrator{ownerBankID: "bank-a"}
	foreignResp, err := statusApp(foreign, "bank-b", true, "").
		Test(httptest.NewRequest(http.MethodGet, "/swap/SWAP-001", nil), -1)
	require.NoError(t, err)
	defer foreignResp.Body.Close()
	foreignBody := readBody(t, foreignResp)

	absent := &mockOwnedSwapOrchestrator{ownerBankID: "bank-a", absent: true}
	absentResp, err := statusApp(absent, "bank-b", true, "").
		Test(httptest.NewRequest(http.MethodGet, "/swap/SWAP-404", nil), -1)
	require.NoError(t, err)
	defer absentResp.Body.Close()
	absentBody := readBody(t, absentResp)

	require.Equal(t, http.StatusNotFound, foreignResp.StatusCode)
	require.Equal(t, http.StatusNotFound, absentResp.StatusCode)
	assert.Equal(t, absentResp.StatusCode, foreignResp.StatusCode,
		"a foreign swap and an absent swap must share a status code")
	assert.Equal(t, absentBody, foreignBody,
		"a foreign swap and an absent swap must share a byte-identical body, or the endpoint "+
			"confirms which swap ids exist")
	assert.Equal(t, absentResp.Header.Get("Content-Type"), foreignResp.Header.Get("Content-Type"))
}

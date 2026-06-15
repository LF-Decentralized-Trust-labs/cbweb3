// Package handlers provides contract tests for CrossCurrencyBridgeOutHandler
// (R2-CR-6: on-chain swap verification + replay protection).
package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- stubs ---

// stubBurnEnqueuer records the arguments of the last EnqueueBurnAfterSwap call.
type stubBurnEnqueuer struct {
	calls    int
	amount   string
	extras   []string
	failWith error
}

func (s *stubBurnEnqueuer) EnqueueBurnAfterSwap(
	_ context.Context,
	_, _, _, _, amount, correlationID string,
	extras ...string,
) (*services.BridgePositionResult, error) {
	s.calls++
	s.amount = amount
	s.extras = extras
	if s.failWith != nil {
		return nil, s.failWith
	}
	return &services.BridgePositionResult{PositionID: "pos-new", BridgeState: "ACTIVE"}, nil
}

type stubBeneficiaryResolver struct {
	addr string
	err  error
}

func (s *stubBeneficiaryResolver) ResolveWalletAddress(_ context.Context, _ string) (string, error) {
	return s.addr, s.err
}

// stubSwapVerifier returns a fixed verified swap or an error.
type stubSwapVerifier struct {
	swap  *handlers.VerifiedSwap
	err   error
	calls int
}

func (s *stubSwapVerifier) VerifySwap(_ context.Context, _ string) (*handlers.VerifiedSwap, error) {
	s.calls++
	return s.swap, s.err
}

// stubDuplicateFinder simulates the swap_tx_hash idempotency lookup.
type stubDuplicateFinder struct {
	existing *services.BridgePositionResult
	err      error
}

func (s *stubDuplicateFinder) FindBySwapTxHash(_ context.Context, _ string) (*services.BridgePositionResult, error) {
	return s.existing, s.err
}

const (
	testWToken    = "0x4e72770760c011647d4873f60a3cf6cdea896cd8"
	testRecipient = "0xaaaa770760c011647d4873f60a3cf6cdea896cd8"
)

// validBody is a relay payload whose swap_sender_address deliberately differs from the
// verified on-chain recipient, so tests can assert the JSON value is never trusted.
func validBody() string {
	return `{
		"correlation_id": "corr-1",
		"swap_tx_hash": "0xfeed",
		"pool_pair": "W-BRL-ARS",
		"amount_out": "60",
		"beneficiary_bank_id": "bank-b",
		"spoke_out": "spoke-b",
		"swap_sender_address": "0xattacker0000000000000000000000000000dead"
	}`
}

func verifiedSwapOK() *handlers.VerifiedSwap {
	return &handlers.VerifiedSwap{
		TokenOut:  testWToken,
		AmountOut: "60",
		Recipient: testRecipient,
	}
}

type bridgeOutFixture struct {
	app      *fiber.App
	enqueuer *stubBurnEnqueuer
	verifier *stubSwapVerifier
	finder   *stubDuplicateFinder
}

func newBridgeOutFixture(verifier *stubSwapVerifier, finder *stubDuplicateFinder) *bridgeOutFixture {
	enqueuer := &stubBurnEnqueuer{}
	h := handlers.NewCrossCurrencyBridgeOutHandler(
		enqueuer,
		&stubBeneficiaryResolver{addr: "0xbeneficiary00000000000000000000000000001"},
		testWToken,
		"0xf12b5dd4ead5f743c6baa640b0216200e89b60da",
		"spoke-b",
	)
	if verifier != nil || finder != nil {
		h = h.WithSwapVerification(verifier, finder)
	}
	app := fiber.New()
	app.Post("/internal/amm/cross-currency-bridge-out", h.HandleBridgeOut)
	return &bridgeOutFixture{app: app, enqueuer: enqueuer, verifier: verifier, finder: finder}
}

func postBridgeOut(t *testing.T, app *fiber.App, body string) (*http.Response, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/internal/amm/cross-currency-bridge-out", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var parsed map[string]any
	if len(raw) > 0 {
		require.NoError(t, json.Unmarshal(raw, &parsed), "body: %s", raw)
	}
	return resp, parsed
}

// swap_tx_hash is now a required field: replays cannot dodge verification by omitting it.
func TestBridgeOut_MissingSwapTxHashRejected(t *testing.T) {
	f := newBridgeOutFixture(&stubSwapVerifier{swap: verifiedSwapOK()}, &stubDuplicateFinder{})

	body := `{"correlation_id":"corr-1","amount_out":"60","beneficiary_bank_id":"bank-b"}`
	resp, _ := postBridgeOut(t, f.app, body)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, 0, f.enqueuer.calls)
}

// Without a configured verifier the endpoint fails closed: no burn/mint on trust alone.
func TestBridgeOut_FailsClosedWithoutVerifier(t *testing.T) {
	f := newBridgeOutFixture(nil, nil)

	resp, _ := postBridgeOut(t, f.app, validBody())

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Equal(t, 0, f.enqueuer.calls)
}

// A swap that cannot be verified on the Hub (missing, reverted, wrong contract) is rejected.
func TestBridgeOut_UnverifiableSwapRejected(t *testing.T) {
	f := newBridgeOutFixture(&stubSwapVerifier{err: fmt.Errorf("tx not found")}, &stubDuplicateFinder{})

	resp, body := postBridgeOut(t, f.app, validBody())

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Equal(t, "SWAP_NOT_VERIFIED", body["code"])
	assert.Equal(t, 0, f.enqueuer.calls)
}

// The swap output token must be this CB's W-token; a swap of some other pair cannot
// trigger a tCeBM mint.
func TestBridgeOut_WrongOutputTokenRejected(t *testing.T) {
	swap := verifiedSwapOK()
	swap.TokenOut = "0x9999999999999999999999999999999999999999"
	f := newBridgeOutFixture(&stubSwapVerifier{swap: swap}, &stubDuplicateFinder{})

	resp, body := postBridgeOut(t, f.app, validBody())

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Equal(t, "SWAP_TOKEN_MISMATCH", body["code"])
	assert.Equal(t, 0, f.enqueuer.calls)
}

// The relay-claimed amount must match the on-chain LogSwap amount.
func TestBridgeOut_AmountMismatchRejected(t *testing.T) {
	swap := verifiedSwapOK()
	swap.AmountOut = "61"
	f := newBridgeOutFixture(&stubSwapVerifier{swap: swap}, &stubDuplicateFinder{})

	resp, body := postBridgeOut(t, f.app, validBody())

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Equal(t, "AMOUNT_MISMATCH", body["code"])
	assert.Equal(t, 0, f.enqueuer.calls)
}

// A zero (or otherwise non-positive) amount_out is rejected before any verification or
// enqueue, preventing a phantom zero-amount bridge-out position (R2-CR-6 review).
func TestBridgeOut_ZeroAmountRejected(t *testing.T) {
	swap := verifiedSwapOK()
	swap.AmountOut = "0"
	f := newBridgeOutFixture(&stubSwapVerifier{swap: swap}, &stubDuplicateFinder{})

	body := `{
		"correlation_id": "corr-1",
		"swap_tx_hash": "0xfeed",
		"amount_out": "0",
		"beneficiary_bank_id": "bank-b",
		"spoke_out": "spoke-b"
	}`
	resp, parsed := postBridgeOut(t, f.app, body)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "INVALID_AMOUNT", parsed["code"])
	assert.Equal(t, 0, f.enqueuer.calls)
	assert.Equal(t, 0, f.verifier.calls, "zero amount must be rejected before touching the Hub")
}

// Happy path: the burn is enqueued with the on-chain amount and the on-chain recipient
// as burn-from — never the client-supplied swap_sender_address.
func TestBridgeOut_BurnsFromVerifiedRecipient(t *testing.T) {
	f := newBridgeOutFixture(&stubSwapVerifier{swap: verifiedSwapOK()}, &stubDuplicateFinder{})

	resp, body := postBridgeOut(t, f.app, validBody())

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	assert.Equal(t, "pos-new", body["position_id"])
	require.Equal(t, 1, f.enqueuer.calls)
	assert.Equal(t, "60", f.enqueuer.amount)
	require.GreaterOrEqual(t, len(f.enqueuer.extras), 3)
	assert.Equal(t, testRecipient, f.enqueuer.extras[0], "burn-from must be the verified on-chain recipient")
	assert.NotContains(t, f.enqueuer.extras[0], "attacker")
	assert.Equal(t, "0xfeed", f.enqueuer.extras[2], "swap_tx_hash must be persisted for idempotency")
}

// Replays of an already-processed swap_tx_hash are acknowledged idempotently:
// the existing position is returned and no second burn/mint is enqueued.
func TestBridgeOut_ReplayIsIdempotent(t *testing.T) {
	finder := &stubDuplicateFinder{existing: &services.BridgePositionResult{PositionID: "pos-existing", BridgeState: "RELEASED"}}
	f := newBridgeOutFixture(&stubSwapVerifier{swap: verifiedSwapOK()}, finder)

	resp, body := postBridgeOut(t, f.app, validBody())

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "duplicate", body["status"])
	assert.Equal(t, "pos-existing", body["position_id"])
	assert.Equal(t, 0, f.enqueuer.calls, "a replay must never enqueue a second burn/mint")
}

// Beneficiary resolution failures still reject the request (pre-existing behavior).
func TestBridgeOut_UnknownBeneficiaryRejected(t *testing.T) {
	enqueuer := &stubBurnEnqueuer{}
	h := handlers.NewCrossCurrencyBridgeOutHandler(
		enqueuer,
		&stubBeneficiaryResolver{err: fmt.Errorf("not found")},
		testWToken,
		"0xf12b5dd4ead5f743c6baa640b0216200e89b60da",
		"spoke-b",
	).WithSwapVerification(&stubSwapVerifier{swap: verifiedSwapOK()}, &stubDuplicateFinder{})
	app := fiber.New()
	app.Post("/internal/amm/cross-currency-bridge-out", h.HandleBridgeOut)

	resp, body := postBridgeOut(t, app, validBody())

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Equal(t, "BENEFICIARY_NOT_FOUND", body["code"])
	assert.Equal(t, 0, enqueuer.calls)
}

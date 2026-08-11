// SPDX-License-Identifier: Apache-2.0

// Package handlers provides contract tests for CrossCurrencyResidueHandler: the return of
// the slippage buffer that bridge-in had to move but the AMM swap did not consume.
//
// The load-bearing property under test is that the returned amount is *derived*, never
// accepted: it comes from the CB's own bridge-in position minus the on-chain LogSwap
// amount_in, so a compromised or buggy caller cannot ask for more than it left behind.
package handlers_test

import (
	"context"
	"encoding/json"
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

// stubResidueEnqueuer records the arguments of the last EnqueueResidueReturn call.
type stubResidueEnqueuer struct {
	calls       int
	amount      string
	burnFrom    string
	beneficiary string
	swapTxHash  string
	parentID    string
	spoke       string
	failWith    error
}

func (s *stubResidueEnqueuer) EnqueueResidueReturn(
	_ context.Context,
	_, spokeNetwork, _, _, amount, _ string,
	burnFromHubAddress, beneficiarySpokeAddress, swapTxHash, parentPositionID string,
) (*services.BridgePositionResult, error) {
	s.calls++
	s.amount = amount
	s.spoke = spokeNetwork
	s.burnFrom = burnFromHubAddress
	s.beneficiary = beneficiarySpokeAddress
	s.swapTxHash = swapTxHash
	s.parentID = parentPositionID
	if s.failWith != nil {
		return nil, s.failWith
	}
	return &services.BridgePositionResult{PositionID: "residue-pos-1", BridgeState: "ACTIVE"}, nil
}

type stubResiduePositionReader struct {
	pos *services.BridgePositionDetail
	err error
}

func (s *stubResiduePositionReader) GetPosition(_ context.Context, _ string) (*services.BridgePositionDetail, error) {
	return s.pos, s.err
}

type stubResidueDuplicateFinder struct {
	existing *services.BridgePositionResult
	err      error
	calls    int
}

func (s *stubResidueDuplicateFinder) FindResidueBySwapTxHash(_ context.Context, _ string) (*services.BridgePositionResult, error) {
	s.calls++
	return s.existing, s.err
}

const (
	testSourceWToken = "0x4e72770760c011647d4873f60a3cf6cdea896cd8"
	testSwapSender   = "0xbbbb770760c011647d4873f60a3cf6cdea896cd8"
	testPayerWallet  = "0xcccc770760c011647d4873f60a3cf6cdea896cd8"
)

func residueBody() string {
	return `{
		"correlation_id": "corr-1",
		"swap_tx_hash": "0xfeed",
		"pool_pair": "W-BRL-ARS",
		"bridge_in_position_id": "pos-bridge-in-1",
		"payer_bank_id": "bank-a",
		"spoke_in": "spoke-brl"
	}`
}

// bridgeInPositionOK is the CB's own record of Step 1: it moved 1000 W-BRL to the Hub,
// minted to the initiating gateway's swap signer.
func bridgeInPositionOK() *services.BridgePositionDetail {
	return &services.BridgePositionDetail{
		PositionID:       "pos-bridge-in-1",
		OwnerBankID:      "bank-a",
		SpokeNetwork:     "spoke-brl",
		MirroredAsset:    testSourceWToken,
		MirroredAmount:   "1000",
		BridgeState:      "ACTIVE",
		MintToHubAddress: testSwapSender,
		Leg:              "SETTLEMENT",
	}
}

// verifiedInputSwapOK models a swap that paid 800 of the 1000 bridged in — residue 200.
func verifiedInputSwapOK() *handlers.VerifiedSwap {
	return &handlers.VerifiedSwap{
		User:     testSwapSender,
		TokenIn:  testSourceWToken,
		AmountIn: "800",
	}
}

type residueFixture struct {
	app      *fiber.App
	enqueuer *stubResidueEnqueuer
	reader   *stubResiduePositionReader
}

func newResidueFixture(
	verifier handlers.SwapVerifierIface,
	finder handlers.ResidueDuplicateFinderIface,
	pos *services.BridgePositionDetail,
) *residueFixture {
	enqueuer := &stubResidueEnqueuer{}
	reader := &stubResiduePositionReader{pos: pos}
	h := handlers.NewCrossCurrencyResidueHandler(
		enqueuer,
		reader,
		&stubBeneficiaryResolver{addr: testPayerWallet},
		testSourceWToken,
		"0xf12b5dd4ead5f743c6baa640b0216200e89b60da",
		"spoke-brl",
	)
	if verifier != nil || finder != nil {
		h = h.WithSwapVerification(verifier, finder)
	}
	app := fiber.New()
	app.Post("/internal/amm/cross-currency-residue-return", asVerifiedCaller("bank-a"), h.HandleResidueReturn)
	return &residueFixture{app: app, enqueuer: enqueuer, reader: reader}
}

func postResidue(t *testing.T, app *fiber.App, body string) (*http.Response, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/internal/amm/cross-currency-residue-return", strings.NewReader(body))
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

// The residue is derived from the CB's own records, not requested: bridged 1000 − consumed
// 800 = 200, delivered to the payer's registry-resolved wallet, burned from the verified
// on-chain swap sender.
func TestResidue_DerivesAmountFromPositionAndLogSwap(t *testing.T) {
	f := newResidueFixture(
		&stubSwapVerifier{swap: verifiedInputSwapOK()},
		&stubResidueDuplicateFinder{},
		bridgeInPositionOK(),
	)

	resp, body := postResidue(t, f.app, residueBody())

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	assert.Equal(t, "200", body["amount"])
	require.Equal(t, 1, f.enqueuer.calls)
	assert.Equal(t, "200", f.enqueuer.amount)
	assert.Equal(t, testSwapSender, f.enqueuer.burnFrom, "must burn from the verified swap sender")
	assert.Equal(t, testPayerWallet, f.enqueuer.beneficiary, "must deliver to the resolved payer wallet")
	assert.Equal(t, "0xfeed", f.enqueuer.swapTxHash)
	assert.Equal(t, "pos-bridge-in-1", f.enqueuer.parentID, "residue leg must link to the bridge-in it corrects")
	assert.Equal(t, "spoke-brl", f.enqueuer.spoke)
}

// Without a verifier the endpoint fails closed: returning value on the caller's word alone
// is the vulnerability R2-CR-6 exists to prevent.
func TestResidue_FailsClosedWithoutVerifier(t *testing.T) {
	f := newResidueFixture(nil, nil, bridgeInPositionOK())

	resp, body := postResidue(t, f.app, residueBody())

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Equal(t, "VERIFIER_NOT_CONFIGURED", body["code"])
	assert.Equal(t, 0, f.enqueuer.calls)
}

// A swap whose *input* was some other currency's W-token is not this CB's business.
func TestResidue_RejectsForeignInputToken(t *testing.T) {
	swap := verifiedInputSwapOK()
	swap.TokenIn = "0x9999770760c011647d4873f60a3cf6cdea896cd8"
	f := newResidueFixture(&stubSwapVerifier{swap: swap}, &stubResidueDuplicateFinder{}, bridgeInPositionOK())

	resp, body := postResidue(t, f.app, residueBody())

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Equal(t, "RESIDUE_TOKEN_MISMATCH", body["code"])
	assert.Equal(t, 0, f.enqueuer.calls)
}

// A caller cannot claim another bank's bridge-in position to collect its residue.
func TestResidue_RejectsPositionOfAnotherBank(t *testing.T) {
	pos := bridgeInPositionOK()
	pos.OwnerBankID = "bank-z"
	f := newResidueFixture(&stubSwapVerifier{swap: verifiedInputSwapOK()}, &stubResidueDuplicateFinder{}, pos)

	resp, body := postResidue(t, f.app, residueBody())

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Equal(t, "POSITION_OWNER_MISMATCH", body["code"])
	assert.Equal(t, 0, f.enqueuer.calls)
}

// TestResidue_ForeignPositionLeaksNothingFromTheReplayGuard pins the check order. The replay
// answer carries the refunding position's id and bridge_state, so consulting it before
// ownership would tell a caller that names another bank's swap that the swap exists and has
// already been refunded — information it has no claim to.
func TestResidue_ForeignPositionLeaksNothingFromTheReplayGuard(t *testing.T) {
	pos := bridgeInPositionOK()
	pos.OwnerBankID = "bank-z"
	finder := &stubResidueDuplicateFinder{existing: &services.BridgePositionResult{
		PositionID: "residue-of-another-bank", BridgeState: "RELEASED",
	}}
	f := newResidueFixture(&stubSwapVerifier{swap: verifiedInputSwapOK()}, finder, pos)

	resp, body := postResidue(t, f.app, residueBody())

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Equal(t, "POSITION_OWNER_MISMATCH", body["code"])
	assert.NotContains(t, body, "position_id", "a foreign caller must learn nothing about the refunding position")
	assert.NotContains(t, body, "bridge_state")
	assert.Zero(t, finder.calls, "the replay lookup must not run before ownership is established")
	assert.Equal(t, 0, f.enqueuer.calls)
}

// The residue may only be reclaimed from the address the CB actually minted to.
func TestResidue_RejectsMismatchedSwapSender(t *testing.T) {
	swap := verifiedInputSwapOK()
	swap.User = "0xdead770760c011647d4873f60a3cf6cdea896cd8"
	f := newResidueFixture(&stubSwapVerifier{swap: swap}, &stubResidueDuplicateFinder{}, bridgeInPositionOK())

	resp, body := postResidue(t, f.app, residueBody())

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Equal(t, "SWAP_SENDER_MISMATCH", body["code"])
	assert.Equal(t, 0, f.enqueuer.calls)
}

// A bridge-in that never reached ACTIVE has nothing on the Hub to give back.
func TestResidue_RejectsNonActivePosition(t *testing.T) {
	pos := bridgeInPositionOK()
	pos.BridgeState = "LOCKING"
	f := newResidueFixture(&stubSwapVerifier{swap: verifiedInputSwapOK()}, &stubResidueDuplicateFinder{}, pos)

	resp, body := postResidue(t, f.app, residueBody())

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Equal(t, "POSITION_NOT_ACTIVE", body["code"])
	assert.Equal(t, 0, f.enqueuer.calls)
}

// Spending more than was bridged in means the swap drew on the shared Hub signer's other
// balances. That is a reconciliation event, not a refund — refuse rather than pay out.
func TestResidue_RefusesWhenSwapConsumedMoreThanBridged(t *testing.T) {
	swap := verifiedInputSwapOK()
	swap.AmountIn = "1200" // > the 1000 bridged in
	f := newResidueFixture(&stubSwapVerifier{swap: swap}, &stubResidueDuplicateFinder{}, bridgeInPositionOK())

	resp, body := postResidue(t, f.app, residueBody())

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Equal(t, "RESIDUE_INCONSISTENT", body["code"])
	assert.Equal(t, 0, f.enqueuer.calls)
}

// A swap that consumed exactly the cap leaves nothing to return.
func TestResidue_NoResidueWhenFullCapConsumed(t *testing.T) {
	swap := verifiedInputSwapOK()
	swap.AmountIn = "1000"
	f := newResidueFixture(&stubSwapVerifier{swap: swap}, &stubResidueDuplicateFinder{}, bridgeInPositionOK())

	resp, body := postResidue(t, f.app, residueBody())

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "no_residue", body["status"])
	assert.Equal(t, 0, f.enqueuer.calls)
}

// Replay: the same swap cannot be refunded twice.
func TestResidue_ReplayReturnsExistingPosition(t *testing.T) {
	existing := &services.BridgePositionResult{PositionID: "residue-pos-earlier", BridgeState: "RELEASED"}
	f := newResidueFixture(
		&stubSwapVerifier{swap: verifiedInputSwapOK()},
		&stubResidueDuplicateFinder{existing: existing},
		bridgeInPositionOK(),
	)

	resp, body := postResidue(t, f.app, residueBody())

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "duplicate", body["status"])
	assert.Equal(t, "residue-pos-earlier", body["position_id"])
	assert.Equal(t, 0, f.enqueuer.calls, "a replay must not enqueue a second return")
}

// A residue leg is not a valid parent for another residue leg.
func TestResidue_RejectsResidueLegAsParent(t *testing.T) {
	pos := bridgeInPositionOK()
	pos.Leg = "RESIDUE"
	f := newResidueFixture(&stubSwapVerifier{swap: verifiedInputSwapOK()}, &stubResidueDuplicateFinder{}, pos)

	resp, body := postResidue(t, f.app, residueBody())

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	assert.Equal(t, "POSITION_LEG_MISMATCH", body["code"])
	assert.Equal(t, 0, f.enqueuer.calls)
}

func TestResidue_RequiresBridgeInPositionID(t *testing.T) {
	f := newResidueFixture(&stubSwapVerifier{swap: verifiedInputSwapOK()}, &stubResidueDuplicateFinder{}, bridgeInPositionOK())

	body := `{"correlation_id":"corr-1","swap_tx_hash":"0xfeed","payer_bank_id":"bank-a"}`
	resp, _ := postResidue(t, f.app, body)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, 0, f.enqueuer.calls)
}

// The residue return sends value back to the named bank's wallet. Ownership of the position was
// already checked, but against the request's own payer_bank_id — which is only a boundary once that
// id is known to belong to the caller.
func TestResidue_RefusesACallerActingForAnotherBank(t *testing.T) {
	enqueuer := &stubResidueEnqueuer{}
	reader := &stubResiduePositionReader{pos: bridgeInPositionOK()}
	h := handlers.NewCrossCurrencyResidueHandler(
		enqueuer,
		reader,
		&stubBeneficiaryResolver{addr: testPayerWallet},
		testSourceWToken,
		"0xf12b5dd4ead5f743c6baa640b0216200e89b60da",
		"spoke-brl",
	).WithSwapVerification(&stubSwapVerifier{swap: verifiedInputSwapOK()}, nil)

	app := fiber.New()
	app.Post("/internal/amm/cross-currency-residue-return", asVerifiedCaller("bank-b"), h.HandleResidueReturn)

	resp, out := postResidue(t, app, residueBody())

	require.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.Equal(t, "RELAY_CALLER_BANK_MISMATCH", out["code"])
	assert.Zero(t, enqueuer.calls, "no value goes back to a bank the caller is not")
}

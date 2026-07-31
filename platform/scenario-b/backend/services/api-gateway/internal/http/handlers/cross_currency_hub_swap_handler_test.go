// SPDX-License-Identifier: Apache-2.0

// Package handlers provides contract tests for CrossCurrencyHubSwapHandler: the sovereign
// delegation of the Step 2 Hub AMM swap from a commercial bank to its issuing CB.
//
// The load-bearing properties under test:
//   - the spend ceiling comes from the bridge-in position the CB itself minted, so a caller
//     cannot reach beyond what was bridged for it (which is what keeps one bank's swap off
//     another bank's balance on the CB's shared Hub address);
//   - a bridge-in position funds at most one swap (the trade is not idempotent on-chain);
//   - ownership is rejected before the replay lookup, so a caller learns nothing about
//     another bank's position;
//   - the executing CB's own Hub address is reported back, because that is where the output
//     lands and where the beneficiary CB must burn from.
package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
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

const cbHubSigner = "0xCB00000000000000000000000000000000000001"

// --- stubs ---

type stubHubSwapExecutor struct {
	calls    int
	lastReq  services.SwapRequest
	amountIn string
	txHash   string
	failWith error
}

func (s *stubHubSwapExecutor) Execute(_ context.Context, req services.SwapRequest) (*services.SwapResult, error) {
	s.calls++
	s.lastReq = req
	if s.failWith != nil {
		return nil, s.failWith
	}
	return &services.SwapResult{TxHash: s.txHash, AmountIn: s.amountIn}, nil
}

type stubHubSwapPositionReader struct {
	pos *services.BridgePositionDetail
	err error
}

func (s *stubHubSwapPositionReader) GetPosition(_ context.Context, _ string) (*services.BridgePositionDetail, error) {
	return s.pos, s.err
}

type stubHubSwapRecorder struct {
	existing   *services.HubSwapRecord
	findErr    error
	recorded   *services.HubSwapRecord
	recordErr  error
	conflictTo *services.HubSwapRecord // returned by Record to simulate a lost race
	findCalls  int
}

func (s *stubHubSwapRecorder) FindByBridgeInPosition(_ context.Context, _ string) (*services.HubSwapRecord, error) {
	s.findCalls++
	return s.existing, s.findErr
}

func (s *stubHubSwapRecorder) Record(_ context.Context, rec *services.HubSwapRecord) (*services.HubSwapRecord, error) {
	if s.recordErr != nil {
		return nil, s.recordErr
	}
	if s.conflictTo != nil {
		return s.conflictTo, nil
	}
	s.recorded = rec
	return rec, nil
}

type stubHubSwapPayerResolver struct {
	wallet string
	err    error
}

func (s *stubHubSwapPayerResolver) ResolveWalletAddress(_ context.Context, _ string) (string, error) {
	return s.wallet, s.err
}

type stubDirectionResolver struct {
	outputIsTokenA bool
	err            error
}

func (s *stubDirectionResolver) OutputIsTokenA(_ context.Context, _, _ string) (bool, error) {
	return s.outputIsTokenA, s.err
}

// --- helpers ---

func activeBridgeInPosition() *services.BridgePositionDetail {
	return &services.BridgePositionDetail{
		PositionID:     "pos-1",
		OwnerBankID:    "bank-a",
		MirroredAsset:  "0xWTOKEN",
		MirroredAmount: "1000",
		BridgeState:    "ACTIVE",
		Leg:            "SETTLEMENT",
	}
}

type hubSwapDeps struct {
	executor  *stubHubSwapExecutor
	reader    *stubHubSwapPositionReader
	recorder  *stubHubSwapRecorder
	payer     *stubHubSwapPayerResolver
	direction *stubDirectionResolver
}

func newHubSwapApp(t *testing.T, d hubSwapDeps) (*fiber.App, hubSwapDeps) {
	t.Helper()
	if d.executor == nil {
		d.executor = &stubHubSwapExecutor{txHash: "0xswap", amountIn: "800"}
	}
	if d.reader == nil {
		d.reader = &stubHubSwapPositionReader{pos: activeBridgeInPosition()}
	}
	if d.recorder == nil {
		d.recorder = &stubHubSwapRecorder{}
	}
	if d.payer == nil {
		d.payer = &stubHubSwapPayerResolver{wallet: "0xBANKA"}
	}
	h := handlers.NewCrossCurrencyHubSwapHandler(
		d.executor, d.reader, d.recorder, d.payer, "0xWTOKEN", cbHubSigner,
	)
	if d.direction != nil {
		h = h.WithDirectionResolver(d.direction)
	}
	app := fiber.New()
	app.Post(services.HubSwapPath, h.HandleHubSwap)
	return app, d
}

func postHubSwap(t *testing.T, app *fiber.App, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, services.HubSwapPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	out := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return resp.StatusCode, out
}

const validHubSwapBody = `{
  "correlation_id":"corr-1",
  "payer_bank_id":"bank-a",
  "beneficiary_bank_id":"bank-b",
  "bridge_in_position_id":"pos-1",
  "pool_pair":"W-BRL-W-COP",
  "target_currency":"COP",
  "amount_out":"500",
  "max_amount_in":"1000"
}`

// --- tests ---

func TestHubSwap_ExecutesAndReportsRealizedCostAndSenderAddress(t *testing.T) {
	app, d := newHubSwapApp(t, hubSwapDeps{})

	status, out := postHubSwap(t, app, validHubSwapBody)

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "executed", out["status"])
	assert.Equal(t, "0xswap", out["swap_tx_hash"])
	// The realized cost comes from the executor (decoded on-chain), not from the request cap.
	assert.Equal(t, "800", out["amount_in"])
	// The output landed on the CB, so the CB must say so — Step 3 burns from here.
	assert.Equal(t, cbHubSigner, out["swap_sender_address"])
	assert.Equal(t, 1, d.executor.calls)
	require.NotNil(t, d.recorder.recorded)
	assert.Equal(t, "pos-1", d.recorder.recorded.BridgeInPositionID)
	assert.Equal(t, "800", d.recorder.recorded.AmountIn)
}

func TestHubSwap_RefusesMaxAmountInAboveBridgedAmount(t *testing.T) {
	// The position funded 1000; asking to spend 1001 would reach into whatever else sits on
	// the CB's Hub address — that is, another bank's bridged W-<source>.
	app, d := newHubSwapApp(t, hubSwapDeps{})

	body := strings.Replace(validHubSwapBody, `"max_amount_in":"1000"`, `"max_amount_in":"1001"`, 1)
	status, out := postHubSwap(t, app, body)

	require.Equal(t, http.StatusUnprocessableEntity, status)
	assert.Equal(t, "MAX_AMOUNT_IN_EXCEEDS_BRIDGED", out["code"])
	assert.Zero(t, d.executor.calls, "no swap may run when the ceiling is not funded")
}

func TestHubSwap_AllowsMaxAmountInEqualToBridgedAmount(t *testing.T) {
	app, d := newHubSwapApp(t, hubSwapDeps{})

	status, _ := postHubSwap(t, app, validHubSwapBody) // max_amount_in == mirrored_amount

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, 1, d.executor.calls)
}

func TestHubSwap_ReplayReturnsRecordedSwapWithoutSwappingAgain(t *testing.T) {
	app, d := newHubSwapApp(t, hubSwapDeps{
		recorder: &stubHubSwapRecorder{existing: &services.HubSwapRecord{
			BridgeInPositionID: "pos-1", SwapTxHash: "0xfirst", AmountIn: "777",
		}},
	})

	status, out := postHubSwap(t, app, validHubSwapBody)

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "duplicate", out["status"])
	assert.Equal(t, "0xfirst", out["swap_tx_hash"])
	assert.Equal(t, "777", out["amount_in"])
	assert.Equal(t, cbHubSigner, out["swap_sender_address"])
	assert.Zero(t, d.executor.calls, "a delegated swap must never run twice for one position")
}

func TestHubSwap_LostRaceReturnsTheWinningRecord(t *testing.T) {
	// Two concurrent delegations: the replay lookup found nothing, but the insert lost the
	// race. The stored record is authoritative.
	app, _ := newHubSwapApp(t, hubSwapDeps{
		recorder: &stubHubSwapRecorder{conflictTo: &services.HubSwapRecord{
			BridgeInPositionID: "pos-1", SwapTxHash: "0xwinner", AmountIn: "790",
		}},
	})

	status, out := postHubSwap(t, app, validHubSwapBody)

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "0xwinner", out["swap_tx_hash"])
	assert.Equal(t, "790", out["amount_in"])
}

func TestHubSwap_RejectsForeignPositionBeforeConsultingReplayGuard(t *testing.T) {
	// A caller naming another bank's position must not learn whether that position was
	// already swapped, so ownership is checked first.
	pos := activeBridgeInPosition()
	pos.OwnerBankID = "bank-z"
	app, d := newHubSwapApp(t, hubSwapDeps{
		reader:   &stubHubSwapPositionReader{pos: pos},
		recorder: &stubHubSwapRecorder{existing: &services.HubSwapRecord{SwapTxHash: "0xsecret"}},
	})

	status, out := postHubSwap(t, app, validHubSwapBody)

	require.Equal(t, http.StatusUnprocessableEntity, status)
	assert.Equal(t, "POSITION_OWNER_MISMATCH", out["code"])
	assert.NotContains(t, out, "swap_tx_hash")
	assert.Zero(t, d.recorder.findCalls, "replay lookup must not run for a foreign position")
	assert.Zero(t, d.executor.calls)
}

func TestHubSwap_RejectsPositionNotActive(t *testing.T) {
	pos := activeBridgeInPosition()
	pos.BridgeState = "LOCKING"
	app, d := newHubSwapApp(t, hubSwapDeps{reader: &stubHubSwapPositionReader{pos: pos}})

	status, out := postHubSwap(t, app, validHubSwapBody)

	require.Equal(t, http.StatusUnprocessableEntity, status)
	assert.Equal(t, "POSITION_NOT_ACTIVE", out["code"])
	assert.Zero(t, d.executor.calls, "nothing is on the Hub to spend before ACTIVE")
}

func TestHubSwap_RejectsPositionInAnotherCBsToken(t *testing.T) {
	pos := activeBridgeInPosition()
	pos.MirroredAsset = "0xSOMEOTHERTOKEN"
	app, d := newHubSwapApp(t, hubSwapDeps{reader: &stubHubSwapPositionReader{pos: pos}})

	status, out := postHubSwap(t, app, validHubSwapBody)

	require.Equal(t, http.StatusUnprocessableEntity, status)
	assert.Equal(t, "POSITION_TOKEN_MISMATCH", out["code"])
	assert.Zero(t, d.executor.calls)
}

func TestHubSwap_RejectsResidueLegAsFundingPosition(t *testing.T) {
	pos := activeBridgeInPosition()
	pos.Leg = "RESIDUE"
	app, d := newHubSwapApp(t, hubSwapDeps{reader: &stubHubSwapPositionReader{pos: pos}})

	status, out := postHubSwap(t, app, validHubSwapBody)

	require.Equal(t, http.StatusUnprocessableEntity, status)
	assert.Equal(t, "POSITION_LEG_MISMATCH", out["code"])
	assert.Zero(t, d.executor.calls)
}

func TestHubSwap_RejectsInactivePayerBank(t *testing.T) {
	// Compliance re-check at payment initiation: on the Hub the trade appears as the CB, so
	// this is what carries bank-level authorisation.
	app, d := newHubSwapApp(t, hubSwapDeps{
		payer: &stubHubSwapPayerResolver{err: errors.New("participant bank_code=\"bank-a\" is not ACTIVE (status=SUSPENDED)")},
	})

	status, out := postHubSwap(t, app, validHubSwapBody)

	require.Equal(t, http.StatusUnprocessableEntity, status)
	assert.Equal(t, "PAYER_NOT_FOUND", out["code"])
	assert.Zero(t, d.executor.calls, "a suspended bank must not trade through the CB's Hub identity")
}

func TestHubSwap_FailsClosedWithoutReplayGuard(t *testing.T) {
	h := handlers.NewCrossCurrencyHubSwapHandler(
		&stubHubSwapExecutor{}, &stubHubSwapPositionReader{pos: activeBridgeInPosition()},
		nil, &stubHubSwapPayerResolver{wallet: "0xBANKA"}, "0xWTOKEN", cbHubSigner,
	)
	app := fiber.New()
	app.Post(services.HubSwapPath, h.HandleHubSwap)

	status, out := postHubSwap(t, app, validHubSwapBody)

	require.Equal(t, http.StatusServiceUnavailable, status)
	assert.Equal(t, "RECORDER_NOT_CONFIGURED", out["code"])
}

func TestHubSwap_FailsClosedWithoutHubSignerAddress(t *testing.T) {
	// Without its own address the CB cannot tell the bank where the output landed, and Step 3
	// would burn from the wrong place.
	h := handlers.NewCrossCurrencyHubSwapHandler(
		&stubHubSwapExecutor{}, &stubHubSwapPositionReader{pos: activeBridgeInPosition()},
		&stubHubSwapRecorder{}, &stubHubSwapPayerResolver{wallet: "0xBANKA"}, "0xWTOKEN", "",
	)
	app := fiber.New()
	app.Post(services.HubSwapPath, h.HandleHubSwap)

	status, out := postHubSwap(t, app, validHubSwapBody)

	require.Equal(t, http.StatusServiceUnavailable, status)
	assert.Equal(t, "HUB_SIGNER_NOT_CONFIGURED", out["code"])
}

func TestHubSwap_ForwardsResolvedCorridorDirection(t *testing.T) {
	app, d := newHubSwapApp(t, hubSwapDeps{direction: &stubDirectionResolver{outputIsTokenA: true}})

	status, _ := postHubSwap(t, app, validHubSwapBody)

	require.Equal(t, http.StatusOK, status)
	assert.True(t, d.executor.lastReq.OutputIsTokenA, "reverse corridor must swap B→A")
	assert.Equal(t, "W-BRL-W-COP", d.executor.lastReq.Pair)
	assert.Equal(t, "500", d.executor.lastReq.AmountOut)
	assert.Equal(t, "1000", d.executor.lastReq.MaxAmountIn)
}

func TestHubSwap_DefaultsToForwardDirectionWhenResolverFails(t *testing.T) {
	app, d := newHubSwapApp(t, hubSwapDeps{
		direction: &stubDirectionResolver{outputIsTokenA: true, err: errors.New("registry unreachable")},
	})

	status, _ := postHubSwap(t, app, validHubSwapBody)

	require.Equal(t, http.StatusOK, status)
	assert.False(t, d.executor.lastReq.OutputIsTokenA)
}

func TestHubSwap_SurfacesExecutionFailureWithoutRecording(t *testing.T) {
	app, d := newHubSwapApp(t, hubSwapDeps{
		executor: &stubHubSwapExecutor{failWith: errors.New("AMM__SlippageExceeded")},
	})

	status, out := postHubSwap(t, app, validHubSwapBody)

	require.Equal(t, http.StatusUnprocessableEntity, status)
	assert.Equal(t, "SWAP_FAILED", out["code"])
	assert.Nil(t, d.recorder.recorded, "a failed swap must not be recorded as done")
}

func TestHubSwap_ReportsExecutedButUnrecordedForReconciliation(t *testing.T) {
	// The trade is on-chain but the record was lost. Reporting failure would invite a second
	// swap; the response says what happened and flags reconciliation instead.
	app, _ := newHubSwapApp(t, hubSwapDeps{
		recorder: &stubHubSwapRecorder{recordErr: errors.New("db down")},
	})

	status, out := postHubSwap(t, app, validHubSwapBody)

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "executed_unrecorded", out["status"])
	assert.Equal(t, "0xswap", out["swap_tx_hash"])
	assert.NotEmpty(t, out["warning"])
}

func TestHubSwap_RejectsMissingRequiredFields(t *testing.T) {
	app, d := newHubSwapApp(t, hubSwapDeps{})

	status, _ := postHubSwap(t, app, `{"correlation_id":"corr-1","payer_bank_id":"bank-a"}`)

	require.Equal(t, http.StatusBadRequest, status)
	assert.Zero(t, d.executor.calls)
}

func TestHubSwap_RejectsNonPositiveAmounts(t *testing.T) {
	for _, tc := range []struct{ name, body, code string }{
		{"zero max_amount_in", strings.Replace(validHubSwapBody, `"max_amount_in":"1000"`, `"max_amount_in":"0"`, 1), "INVALID_MAX_AMOUNT_IN"},
		{"non-numeric max_amount_in", strings.Replace(validHubSwapBody, `"max_amount_in":"1000"`, `"max_amount_in":"abc"`, 1), "INVALID_MAX_AMOUNT_IN"},
		{"zero amount_out", strings.Replace(validHubSwapBody, `"amount_out":"500"`, `"amount_out":"0"`, 1), "INVALID_AMOUNT_OUT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, d := newHubSwapApp(t, hubSwapDeps{})
			status, out := postHubSwap(t, app, tc.body)
			require.Equal(t, http.StatusBadRequest, status)
			assert.Equal(t, tc.code, out["code"])
			assert.Zero(t, d.executor.calls)
		})
	}
}

// SPDX-License-Identifier: Apache-2.0

// Package handlers provides contract tests for GovernanceScenarioBHandler (T093 / FR-030 / FR-044 / SC-026).
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockCBService struct {
	pauseErr   error
	pauseTx    string
	proposeID  string
	proposeTx  string
	proposeErr error
	signTx     string
	signErr    error
	executeErr error
	statusTx   string
	calls      []string
}

func (m *mockCBService) Pause(ctx context.Context, pair, bankID, reason string, sig []byte) (string, error) {
	m.calls = append(m.calls, "Pause")
	return m.pauseTx, m.pauseErr
}
func (m *mockCBService) ProposeResume(ctx context.Context, pair, bankID string, sig []byte) (string, string, error) {
	m.calls = append(m.calls, "ProposeResume")
	return m.proposeID, m.proposeTx, m.proposeErr
}
func (m *mockCBService) SignResume(ctx context.Context, pair, requestID, bankID string, sig []byte) (string, error) {
	m.calls = append(m.calls, "SignResume")
	return m.signTx, m.signErr
}
func (m *mockCBService) ExecuteResume(ctx context.Context, pair, requestID string) error {
	m.calls = append(m.calls, "ExecuteResume")
	return m.executeErr
}
func (m *mockCBService) GetStatus(ctx context.Context, pair string) (*services.CircuitBreakerStatus, error) {
	m.calls = append(m.calls, "GetStatus")
	return &services.CircuitBreakerStatus{Pair: pair, State: "LIVE", TxHash: m.statusTx}, nil
}

func newGovFiber(svc handlers.CircuitBreakerServiceIface) *fiber.App {
	app := fiber.New()
	h := handlers.NewGovernanceScenarioBHandler(svc)
	app.Post("/pause", h.PauseCircuitBreaker)
	app.Post("/resume-request", h.ProposeResume)
	app.Post("/resume-sign", h.SignResume)
	app.Get("/status", h.GetCircuitBreakerStatus)
	return app
}

// (a) pause success -> HTTP 200 + HALTED
func TestGovHandler_Pause_Success(t *testing.T) {
	svc := &mockCBService{}
	app := newGovFiber(svc)

	body, _ := json.Marshal(map[string]string{
		"pair":        "BRL-USD",
		"bank_id":     "cb-bra",
		"reason_code": "INCIDENT",
	})
	req := httptest.NewRequest(http.MethodPost, "/pause", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	payload, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(payload), "HALTED")
	assert.Contains(t, svc.calls, "Pause")
}

// (b) pause missing required field -> HTTP 400
func TestGovHandler_Pause_MissingFields(t *testing.T) {
	svc := &mockCBService{}
	app := newGovFiber(svc)

	body, _ := json.Marshal(map[string]string{"pair": "BRL-USD"})
	req := httptest.NewRequest(http.MethodPost, "/pause", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// (c) resume-sign with insufficient quorum -> HTTP 422 + CIRCUIT_BREAKER_RESUME_DISPUTED
func TestGovHandler_ResumeSign_InsufficientQuorum(t *testing.T) {
	svc := &mockCBService{signErr: errors.New("quorum not reached")}
	app := newGovFiber(svc)

	body, _ := json.Marshal(map[string]string{
		"pair":       "BRL-USD",
		"request_id": "req-123",
		"bank_id":    "cb-bra",
	})
	req := httptest.NewRequest(http.MethodPost, "/resume-sign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	payload, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(payload), "CIRCUIT_BREAKER_RESUME_DISPUTED")
}

// (d) resume-sign with quorum complete -> HTTP 200 + LIVE
func TestGovHandler_ResumeSign_QuorumComplete(t *testing.T) {
	svc := &mockCBService{} // both SignResume and ExecuteResume succeed
	app := newGovFiber(svc)

	body, _ := json.Marshal(map[string]string{
		"pair":       "BRL-USD",
		"request_id": "req-123",
		"bank_id":    "cb-arg",
	})
	req := httptest.NewRequest(http.MethodPost, "/resume-sign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	payload, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(payload), "LIVE")
	assert.Contains(t, svc.calls, "SignResume")
	assert.Contains(t, svc.calls, "ExecuteResume")
}

// Status endpoint returns JSON envelope
func TestGovHandler_Status(t *testing.T) {
	svc := &mockCBService{}
	app := newGovFiber(svc)

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	payload, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(payload), "state")
}

// decodeCBJSON reads a response body into a generic map so a test can assert on key
// *presence*, which is what contract rule C-1 is about — `""` is not the same as absent.
func decodeCBJSON(t *testing.T, body io.Reader) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.NewDecoder(body).Decode(&out))
	return out
}

func postCBJSON(t *testing.T, app *fiber.App, path string, payload map[string]string) *http.Response {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	return resp
}

// T011 / FR-002 / C-1: tx_hash is present in all four responses when chain-wired.
func TestGovHandler_TxHash_PresentWhenChainWired(t *testing.T) {
	svc := &mockCBService{
		pauseTx:   "0xpause111",
		proposeID: "0xproposal222",
		proposeTx: "0xtx333",
		signTx:    "0xsign444",
		statusTx:  "0xstatus555",
	}
	app := newGovFiber(svc)

	pause := decodeCBJSON(t, postCBJSON(t, app, "/pause", map[string]string{
		"pair": "BRL-USD", "bank_id": "cb-bra", "reason_code": "INCIDENT",
	}).Body)
	assert.Equal(t, "HALTED", pause["state"])
	assert.Equal(t, "0xpause111", pause["tx_hash"])

	propose := decodeCBJSON(t, postCBJSON(t, app, "/resume-request", map[string]string{
		"pair": "BRL-USD", "bank_id": "cb-bra",
	}).Body)
	assert.Equal(t, "RESUME_PENDING", propose["state"])
	assert.Equal(t, "0xproposal222", propose["request_id"])
	assert.Equal(t, "0xtx333", propose["tx_hash"])
	// Rule D-3: the proposal id a co-signer must submit is NOT the transaction hash.
	assert.NotEqual(t, propose["request_id"], propose["tx_hash"],
		"request_id and tx_hash are different identifiers and both are required")

	sign := decodeCBJSON(t, postCBJSON(t, app, "/resume-sign", map[string]string{
		"pair": "BRL-USD", "request_id": "0xproposal222", "bank_id": "cb-arg",
	}).Body)
	assert.Equal(t, "LIVE", sign["state"])
	assert.Equal(t, "0xsign444", sign["tx_hash"])

	statusReq := httptest.NewRequest(http.MethodGet, "/status?pair=BRL-USD", nil)
	statusResp, err := app.Test(statusReq, -1)
	require.NoError(t, err)
	status := decodeCBJSON(t, statusResp.Body)
	assert.Equal(t, "0xstatus555", status["tx_hash"])
}

// T011 / FR-004: resume-sign still carries this signature's hash when quorum is not yet
// met, i.e. when ExecuteResume reports the resume as not finalised.
func TestGovHandler_TxHash_PresentOnPendingResumeSign(t *testing.T) {
	svc := &mockCBService{signTx: "0xsign444", executeErr: errors.New("quorum not yet met")}
	app := newGovFiber(svc)

	sign := decodeCBJSON(t, postCBJSON(t, app, "/resume-sign", map[string]string{
		"pair": "BRL-USD", "request_id": "0xproposal222", "bank_id": "cb-arg",
	}).Body)
	assert.Equal(t, "RESUME_PENDING", sign["state"])
	assert.Equal(t, "0xsign444", sign["tx_hash"])
}

// T012 / FR-009 / C-1: in no-chain mode tx_hash is absent as a JSON *key*, never "".
// An empty string would be indistinguishable from "a reference exists but was not shown".
func TestGovHandler_TxHash_AbsentKeyInNoChainMode(t *testing.T) {
	svc := &mockCBService{proposeID: "0xproposal222"} // no hashes anywhere
	app := newGovFiber(svc)

	pause := decodeCBJSON(t, postCBJSON(t, app, "/pause", map[string]string{
		"pair": "BRL-USD", "bank_id": "cb-bra", "reason_code": "INCIDENT",
	}).Body)
	assert.Equal(t, "HALTED", pause["state"], "the action must still succeed (C-3)")
	assert.NotContains(t, pause, "tx_hash")

	propose := decodeCBJSON(t, postCBJSON(t, app, "/resume-request", map[string]string{
		"pair": "BRL-USD", "bank_id": "cb-bra",
	}).Body)
	assert.Equal(t, "RESUME_PENDING", propose["state"])
	assert.Equal(t, "0xproposal222", propose["request_id"], "request_id is unaffected by a missing hash")
	assert.NotContains(t, propose, "tx_hash")

	sign := decodeCBJSON(t, postCBJSON(t, app, "/resume-sign", map[string]string{
		"pair": "BRL-USD", "request_id": "0xproposal222", "bank_id": "cb-arg",
	}).Body)
	assert.Equal(t, "LIVE", sign["state"])
	assert.NotContains(t, sign, "tx_hash")

	statusReq := httptest.NewRequest(http.MethodGet, "/status?pair=BRL-USD", nil)
	statusResp, err := app.Test(statusReq, -1)
	require.NoError(t, err)
	status := decodeCBJSON(t, statusResp.Body)
	assert.Equal(t, "LIVE", status["state"])
	assert.NotContains(t, status, "tx_hash")
}

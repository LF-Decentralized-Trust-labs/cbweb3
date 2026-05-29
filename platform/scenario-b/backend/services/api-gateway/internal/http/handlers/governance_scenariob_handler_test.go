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
	proposeID  string
	proposeErr error
	signErr    error
	executeErr error
	calls      []string
}

func (m *mockCBService) Pause(ctx context.Context, pair, bankID, reason string, sig []byte) error {
	m.calls = append(m.calls, "Pause")
	return m.pauseErr
}
func (m *mockCBService) ProposeResume(ctx context.Context, pair, bankID string, sig []byte) (string, error) {
	m.calls = append(m.calls, "ProposeResume")
	return m.proposeID, m.proposeErr
}
func (m *mockCBService) SignResume(ctx context.Context, pair, requestID, bankID string, sig []byte) error {
	m.calls = append(m.calls, "SignResume")
	return m.signErr
}
func (m *mockCBService) ExecuteResume(ctx context.Context, pair, requestID string) error {
	m.calls = append(m.calls, "ExecuteResume")
	return m.executeErr
}
func (m *mockCBService) GetStatus(ctx context.Context, pair string) (*services.CircuitBreakerStatus, error) {
	m.calls = append(m.calls, "GetStatus")
	return &services.CircuitBreakerStatus{Pair: pair, State: "LIVE"}, nil
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

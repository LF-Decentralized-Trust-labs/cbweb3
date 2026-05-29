// Package handlers provides contract tests for the SwapHandler (FR-027 / FR-058 / FR-059).
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSwapService struct {
	resp *services.SwapResult
	err  error
}

func (m *mockSwapService) Execute(_ context.Context, _ services.SwapRequest) (*services.SwapResult, error) {
	return m.resp, m.err
}

func newSwapTestFiber(h *handlers.SwapHandler) *fiber.App {
	app := fiber.New()
	app.Post("/api/v2/amm/swap/exact-output", h.SwapExactOutput)
	return app
}

func validSwapBody() []byte {
	body, _ := json.Marshal(map[string]string{
		"pair":                   "tCeBMA-tCeBMB",
		"amount_out":             "100",
		"max_amount_in":          "110",
		"payer_id":               "bank-a",
		"beneficiary_id":         "bank-b",
		"zk_pointer_payer":       "zkp-payer-hash",
		"zk_pointer_beneficiary": "zkp-bene-hash",
	})
	return body
}

func TestSwapHandler_Success(t *testing.T) {
	svc := &mockSwapService{resp: &services.SwapResult{
		OrderID:     "ORD-001",
		TxHash:      "0xabc",
		AmountIn:    "105",
		State:       "COMPLETED",
		ConfirmedAt: time.Now(),
	}}
	h := handlers.NewSwapHandler(svc)
	app := newSwapTestFiber(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v2/amm/swap/exact-output",
		bytes.NewReader(validSwapBody()))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ORD-001", body["order_id"])
	assert.Equal(t, "COMPLETED", body["state"])
}

func TestSwapHandler_SlippageLimitExceeded(t *testing.T) {
	svc := &mockSwapService{err: &domain.SwapExecError{Code: domain.ErrCodeSlippageLimitExceeded}}
	h := handlers.NewSwapHandler(svc)
	app := newSwapTestFiber(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v2/amm/swap/exact-output",
		bytes.NewReader(validSwapBody()))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, string(domain.ErrCodeSlippageLimitExceeded), body["error_code"])
}

func TestSwapHandler_InsufficientPoolLiquidity(t *testing.T) {
	svc := &mockSwapService{err: &domain.SwapExecError{Code: domain.ErrCodeInsufficientPoolLiquidity}}
	h := handlers.NewSwapHandler(svc)
	app := newSwapTestFiber(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v2/amm/swap/exact-output",
		bytes.NewReader(validSwapBody()))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, string(domain.ErrCodeInsufficientPoolLiquidity), body["error_code"])
}

func TestSwapHandler_ZKValidationFailed(t *testing.T) {
	svc := &mockSwapService{err: &services.ZKValidationError{Msg: "proof invalid"}}
	h := handlers.NewSwapHandler(svc)
	app := newSwapTestFiber(h)

	req := httptest.NewRequest(http.MethodPost, "/api/v2/amm/swap/exact-output",
		bytes.NewReader(validSwapBody()))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, string(domain.ErrCodeZKValidationFailed), body["error_code"])
}

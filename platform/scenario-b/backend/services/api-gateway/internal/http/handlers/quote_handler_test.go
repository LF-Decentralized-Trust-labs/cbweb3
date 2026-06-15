// SPDX-License-Identifier: Apache-2.0

// Package handlers provides contract tests for the QuoteHandler (FR-027).
package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockQuoteService is a test double for QuoteServiceIface.
type mockQuoteService struct {
	resp *services.QuoteResponse
	err  error
}

func (m *mockQuoteService) GetExactOutputQuote(_ context.Context, _, _ string) (*services.QuoteResponse, error) {
	return m.resp, m.err
}

func newTestFiber(handler *handlers.QuoteHandler) *fiber.App {
	app := fiber.New()
	app.Get("/api/v2/amm/quote/exact-output", handler.GetExactOutputQuote)
	return app
}

func TestQuoteHandler_Success(t *testing.T) {
	svc := &mockQuoteService{
		resp: &services.QuoteResponse{
			RequiredInput:  "100.50",
			PriceImpact:    "0.002",
			QuoteTimestamp: time.Now(),
		},
	}
	h := handlers.NewQuoteHandler(svc, nil)
	app := newTestFiber(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/amm/quote/exact-output?pair=tCeBMA-tCeBMB&amount_out=100", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "100.50", body["required_input"])
	assert.Equal(t, "0.002", body["price_impact"])
}

func TestQuoteHandler_MissingPair(t *testing.T) {
	svc := &mockQuoteService{}
	h := handlers.NewQuoteHandler(svc, nil)
	app := newTestFiber(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/amm/quote/exact-output?amount_out=100", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestQuoteHandler_InsufficientLiquidity(t *testing.T) {
	svc := &mockQuoteService{err: fiber.ErrUnprocessableEntity}
	h := handlers.NewQuoteHandler(svc, nil)
	app := newTestFiber(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/amm/quote/exact-output?pair=tCeBMA-tCeBMB&amount_out=999999999", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

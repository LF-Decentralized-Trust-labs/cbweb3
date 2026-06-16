// SPDX-License-Identifier: Apache-2.0

package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// QuoteHandler.GetCrossCurrencyQuote
// ---------------------------------------------------------------------------

type mockQuoteGen struct {
	result *services.QuoteResult
	err    error
}

func (m *mockQuoteGen) GenerateQuote(_ context.Context, _ services.QuoteRequest) (*services.QuoteResult, error) {
	return m.result, m.err
}

func crossQuoteApp(gen handlers.SwapQuoteGeneratorIface) *fiber.App {
	h := handlers.NewQuoteHandler(nil, gen)
	app := fiber.New()
	app.Get("/q/cross", h.GetCrossCurrencyQuote)
	return app
}

func TestCrossCurrencyQuote_Success(t *testing.T) {
	now := time.Now()
	gen := &mockQuoteGen{result: &services.QuoteResult{
		QuoteID: "q-1", PoolPair: "W-BRL-W-ARS", AmountOut: "100", AmountIn: "105",
		EffectiveRate: 0.95, FeeBps: 30, CreatedAt: now, ValidUntil: now.Add(15 * time.Second),
		TimeRemainingSeconds: 15,
	}}
	app := crossQuoteApp(gen)
	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/q/cross?source_currency=BRL&target_currency=ARS&amount_out=100", nil), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCrossCurrencyQuote_MissingParams(t *testing.T) {
	app := crossQuoteApp(&mockQuoteGen{})
	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/q/cross?source_currency=BRL", nil), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestCrossCurrencyQuote_GeneratorError(t *testing.T) {
	app := crossQuoteApp(&mockQuoteGen{err: errors.New("insufficient liquidity")})
	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/q/cross?source_currency=BRL&target_currency=ARS&amount_out=999", nil), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// GetHubLiquidityConfig (env-driven)
// ---------------------------------------------------------------------------

func TestGetHubLiquidityConfig_NotConfigured(t *testing.T) {
	t.Setenv("SOVEREIGN_AMM_ADDRESS", "")
	app := fiber.New()
	app.Get("/hub", handlers.GetHubLiquidityConfig)
	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/hub", nil), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetHubLiquidityConfig_Configured(t *testing.T) {
	t.Setenv("SOVEREIGN_AMM_ADDRESS", "0xamm")
	t.Setenv("SOVEREIGN_PAIR_ID", "W-BRL-ARS")
	app := fiber.New()
	app.Get("/hub", handlers.GetHubLiquidityConfig)
	resp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/hub", nil), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// SwapHandler additional branches
// ---------------------------------------------------------------------------

type swapSvcStub struct {
	result *services.SwapResult
	err    error
}

func (s *swapSvcStub) Execute(_ context.Context, _ services.SwapRequest) (*services.SwapResult, error) {
	return s.result, s.err
}

func swapApp2(svc handlers.SwapServiceIface) *fiber.App {
	h := handlers.NewSwapHandler(svc)
	app := fiber.New()
	app.Post("/swap", h.SwapExactOutput)
	return app
}

func TestSwapHandler_BadBody(t *testing.T) {
	app := swapApp2(&swapSvcStub{})
	resp, _ := app.Test(reqJSON(http.MethodPost, "/swap", "not json"), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestSwapHandler_MissingFields(t *testing.T) {
	app := swapApp2(&swapSvcStub{})
	resp, _ := app.Test(reqJSON(http.MethodPost, "/swap", `{"pair":"P"}`), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestSwapHandler_GenericError(t *testing.T) {
	app := swapApp2(&swapSvcStub{err: errors.New("boom")})
	body := `{"pair":"P","amount_out":"1","max_amount_in":"2","payer_id":"a","beneficiary_id":"b"}`
	resp, _ := app.Test(reqJSON(http.MethodPost, "/swap", body), -1)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// SPDX-License-Identifier: Apache-2.0

// Package handlers provides contract tests for the PoolHandler (FR-028).
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

type mockPoolStatusService struct {
	resp *services.PoolStatusResponse
	err  error
}

func (m *mockPoolStatusService) GetPoolStatus(_ context.Context, _ string) (*services.PoolStatusResponse, error) {
	return m.resp, m.err
}

func newPoolTestFiber(h *handlers.PoolHandler) *fiber.App {
	app := fiber.New()
	app.Get("/api/v2/amm/pool/:pair/status", h.GetPoolStatus)
	return app
}

func TestPoolHandler_NormalRatio(t *testing.T) {
	svc := &mockPoolStatusService{resp: &services.PoolStatusResponse{
		PoolPair:      "tCeBMA-tCeBMB",
		ReserveA:      "500",
		ReserveB:      "500",
		CurrentRatio:  0.50,
		ImbalanceFlag: false,
		UpdatedAt:     time.Now(),
	}}
	h := handlers.NewPoolHandler(svc)
	app := newPoolTestFiber(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/amm/pool/tCeBMA-tCeBMB/status", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, false, body["imbalance_flag"])
}

func TestPoolHandler_ImbalanceRatio(t *testing.T) {
	svc := &mockPoolStatusService{resp: &services.PoolStatusResponse{
		PoolPair:      "tCeBMA-tCeBMB",
		ReserveA:      "720",
		ReserveB:      "280",
		CurrentRatio:  0.72,
		ImbalanceFlag: true,
		UpdatedAt:     time.Now(),
	}}
	h := handlers.NewPoolHandler(svc)
	app := newPoolTestFiber(h)

	req := httptest.NewRequest(http.MethodGet, "/api/v2/amm/pool/tCeBMA-tCeBMB/status", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, true, body["imbalance_flag"])
}

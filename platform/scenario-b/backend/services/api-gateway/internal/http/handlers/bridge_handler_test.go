// SPDX-License-Identifier: Apache-2.0

// Package handlers provides contract tests for BridgeHandler (T072 / FR-029 / SC-015).
package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- stub implementations ---

type stubLockMintService struct{}

func (s *stubLockMintService) LockAndEnqueue(_ context.Context, ownerBankID, spokeNetwork, nativeAsset, mirroredAsset, amount string) (*services.BridgePositionResult, error) {
	return &services.BridgePositionResult{
		PositionID:     "pos-001",
		OwnerBankID:    ownerBankID,
		SpokeNetwork:   spokeNetwork,
		NativeAsset:    nativeAsset,
		MirroredAsset:  mirroredAsset,
		MirroredAmount: amount,
		BridgeState:    "LOCKING",
	}, nil
}

type stubBurnUnlockService struct{}

func (s *stubBurnUnlockService) BurnAndEnqueue(_ context.Context, positionID string) (*services.BridgePositionResult, error) {
	return &services.BridgePositionResult{
		PositionID:  positionID,
		BridgeState: "BURNING",
	}, nil
}

type stubPositionReader struct{}

func (s *stubPositionReader) ListPositions(_ context.Context, stateFilter string) ([]services.BridgePositionResult, error) {
	positions := []services.BridgePositionResult{
		{PositionID: "pos-001", BridgeState: "ACTIVE"},
		{PositionID: "pos-002", BridgeState: "RELEASED"},
	}
	if stateFilter != "" {
		var filtered []services.BridgePositionResult
		for _, p := range positions {
			if p.BridgeState == stateFilter {
				filtered = append(filtered, p)
			}
		}
		return filtered, nil
	}
	return positions, nil
}

func newBridgeTestFiber() *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{BankID: "bank-a"})
		return c.Next()
	})
	h := handlers.NewBridgeHandler(&stubLockMintService{}, &stubBurnUnlockService{}, &stubPositionReader{})
	app.Post("/api/v2/bridge/lock-mint", h.LockMint)
	app.Post("/api/v2/bridge/burn-unlock", h.BurnUnlock)
	app.Get("/api/v2/bridge/positions", h.ListPositions)
	return app
}

// lock-mint with valid body returns 201 and JSON (T072a).
func TestBridgeHandler_LockMintReturnsJSON(t *testing.T) {
	app := newBridgeTestFiber()

	body := `{"owner_bank_id":"bank-a","spoke_network":"spoke-a","native_asset":"tCeBM_BRL","mirrored_asset":"mtCeBM_BRL","amount":"1000"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v2/bridge/lock-mint", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
}

// lock-mint with missing fields returns 400.
func TestBridgeHandler_LockMintMissingFields(t *testing.T) {
	app := newBridgeTestFiber()

	req := httptest.NewRequest(http.MethodPost, "/api/v2/bridge/lock-mint", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestBridgeHandler_BurnUnlockReturnsJSON(t *testing.T) {
	app := newBridgeTestFiber()

	body := `{"position_id":"pos-001"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v2/bridge/burn-unlock", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
}

// ListPositions accepts an optional state filter (FR-033).
func TestBridgeHandler_ListPositions_FilterByState(t *testing.T) {
	app := newBridgeTestFiber()

	req := httptest.NewRequest(http.MethodGet, "/api/v2/bridge/positions?state=ACTIVE", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
}

// lock-mint falls back to configured BANK_CODE when authenticated claims have empty BankID.
func TestBridgeHandler_LockMintUsesFallbackBankCode(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("claims", domain.TokenClaims{})
		return c.Next()
	})
	h := handlers.NewBridgeHandler(&stubLockMintService{}, &stubBurnUnlockService{}, &stubPositionReader{}).SetFallbackBankCode("bank-b")
	app.Post("/api/v2/bridge/lock-mint", h.LockMint)

	body := `{"spoke_network":"spoke-a","native_asset":"tCeBM_BRL","mirrored_asset":"mtCeBM_BRL","amount":"1000"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v2/bridge/lock-mint", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

// SPDX-License-Identifier: Apache-2.0

// Route-authorization tests for the liquidity handler (R2-H-9 / R2-H-10): the
// provider identity on CancelCommit must be derived from the authenticated JWT
// (claims.BankID), not trusted from a query parameter. A cancel whose provider_id
// diverges from the caller's authenticated identity must be rejected.
package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLiquidityService records the provider id passed to CancelCommit and
// otherwise returns benign zero values, so authorization behavior can be asserted
// without a live AMM/registry backend.
type mockLiquidityService struct {
	cancelCalled     bool
	cancelProviderID string
	cancelErr        error
}

func (m *mockLiquidityService) AddLiquidity(context.Context, services.LiquidityProvisionRequest) (*services.LPResult, error) {
	return &services.LPResult{}, nil
}
func (m *mockLiquidityService) RemoveLiquidity(context.Context, services.LiquidityRemoveRequest) (*services.LPResult, error) {
	return &services.LPResult{}, nil
}
func (m *mockLiquidityService) RegisterCommit(context.Context, services.CommitRequest) (*services.CommitResult, error) {
	return &services.CommitResult{}, nil
}
func (m *mockLiquidityService) ListCommits(context.Context, string, string) ([]domain.PoolCommit, error) {
	return nil, nil
}
func (m *mockLiquidityService) GetCommit(context.Context, string) (*domain.PoolCommit, error) {
	return nil, nil
}
func (m *mockLiquidityService) CancelCommit(_ context.Context, _ string, providerID string) error {
	m.cancelCalled = true
	m.cancelProviderID = providerID
	return m.cancelErr
}

func TestCancelCommit_ProviderMismatchRejected(t *testing.T) {
	svc := &mockLiquidityService{}
	h := handlers.NewLiquidityHandler(svc)

	app := fiber.New()
	// Authenticated as bank-a, but the query claims provider_id = bank-b.
	app.Delete("/api/v2/amm/liquidity/commits/:commit_id", injectBankClaims("bank-a"), h.CancelCommit)

	req := httptest.NewRequest(http.MethodDelete, "/api/v2/amm/liquidity/commits/cm1?provider_id=bank-b", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.False(t, svc.cancelCalled, "service must not be invoked on identity mismatch")
}

func TestCancelCommit_DerivesProviderFromClaims(t *testing.T) {
	svc := &mockLiquidityService{}
	h := handlers.NewLiquidityHandler(svc)

	app := fiber.New()
	// Authenticated as bank-a, no query provider_id — identity must be derived from claims.
	app.Delete("/api/v2/amm/liquidity/commits/:commit_id", injectBankClaims("bank-a"), h.CancelCommit)

	req := httptest.NewRequest(http.MethodDelete, "/api/v2/amm/liquidity/commits/cm1", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.True(t, svc.cancelCalled, "service must be invoked")
	assert.Equal(t, "bank-a", svc.cancelProviderID, "provider id must be derived from claims")
}

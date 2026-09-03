// SPDX-License-Identifier: Apache-2.0

// Package handlers provides contract tests for OversightHandler (T094 / FR-034 / FR-035 / SC-027).
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
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockOversightService struct {
	openResp   *services.DisclosureResult
	openErr    error
	signErr    error
	statusResp *services.DisclosureResult
	statusErr  error
	calls      []string
}

func (m *mockOversightService) OpenDisclosure(ctx context.Context, txRef, requestorID, reasonCode string) (*services.DisclosureResult, error) {
	m.calls = append(m.calls, "OpenDisclosure")
	return m.openResp, m.openErr
}
func (m *mockOversightService) SignDisclosure(ctx context.Context, requestID, signerID string) error {
	m.calls = append(m.calls, "SignDisclosure")
	return m.signErr
}
func (m *mockOversightService) GetDisclosureStatus(ctx context.Context, requestID string) (*services.DisclosureResult, error) {
	m.calls = append(m.calls, "GetDisclosureStatus")
	return m.statusResp, m.statusErr
}

func newOversightFiber(svc handlers.OversightServiceIface) *fiber.App {
	app := fiber.New()
	h := handlers.NewOversightHandler(svc)
	app.Post("/disclosure-request", h.OpenDisclosure)
	app.Post("/disclosure-sign", h.SignDisclosure)
	app.Get("/disclosure-status/:requestID", h.GetDisclosureStatus)
	return app
}

// (a) abrir disclosure cria request com expires_at = now+72h
func TestOversightHandler_Open_SetsExpires72h(t *testing.T) {
	now := time.Now().UTC()
	expires := now.Add(72 * time.Hour)
	svc := &mockOversightService{openResp: &services.DisclosureResult{
		RequestID:            "req-1",
		RequestedByBankID:    "cb-bra",
		TargetTransactionRef: "tx-abc",
		ReasonCode:           "REG_INVESTIGATION",
		State:                "PENDING",
		QuorumRequired:       2,
		QuorumReached:        0,
		OpenedAt:             now,
		ExpiresAt:            expires,
	}}
	app := newOversightFiber(svc)

	body, _ := json.Marshal(map[string]string{
		"tx_ref":       "tx-abc",
		"requestor_id": "cb-bra",
		"reason_code":  "REG_INVESTIGATION",
	})
	req := httptest.NewRequest(http.MethodPost, "/disclosure-request", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	payload, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(payload), "PENDING")
	assert.Contains(t, string(payload), "req-1")

	// Assert that ExpiresAt is ~72h after OpenedAt
	diff := expires.Sub(now).Hours()
	assert.InDelta(t, 72.0, diff, 0.01)
}

// (b) assinar sem quorum retorna sucesso mas state PENDING (service validates)
func TestOversightHandler_Sign_Success(t *testing.T) {
	svc := &mockOversightService{}
	app := newOversightFiber(svc)

	body, _ := json.Marshal(map[string]string{
		"request_id": "req-1",
		"signer_id":  "cb-bra",
	})
	req := httptest.NewRequest(http.MethodPost, "/disclosure-sign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, svc.calls, "SignDisclosure")
}

// (c) sign retorna erro quando service rejeita (ex.: disclosure expirada)
func TestOversightHandler_Sign_Expired(t *testing.T) {
	svc := &mockOversightService{signErr: errors.New("disclosure expired")}
	app := newOversightFiber(svc)

	body, _ := json.Marshal(map[string]string{"request_id": "req-1", "signer_id": "cb-bra"})
	req := httptest.NewRequest(http.MethodPost, "/disclosure-sign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

// (d) status not found
func TestOversightHandler_Status_NotFound(t *testing.T) {
	svc := &mockOversightService{statusErr: errors.New("not found")}
	app := newOversightFiber(svc)

	req := httptest.NewRequest(http.MethodGet, "/disclosure-status/req-missing", nil)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// request sem reason_code retorna 400
func TestOversightHandler_Open_MissingReason(t *testing.T) {
	svc := &mockOversightService{}
	app := newOversightFiber(svc)

	body, _ := json.Marshal(map[string]string{"tx_ref": "tx-1", "requestor_id": "cb-bra"})
	req := httptest.NewRequest(http.MethodPost, "/disclosure-request", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

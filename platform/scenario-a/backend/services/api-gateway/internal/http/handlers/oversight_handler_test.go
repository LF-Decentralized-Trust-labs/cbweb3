// SPDX-License-Identifier: Apache-2.0

// Tests for OversightHandler (T037 / FR-034/FR-035 / US3). Must FAIL before T041.
package handlers

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

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

type mockOversightSvc struct {
	openResp   *services.DisclosureResult
	openErr    error
	signErr    error
	statusResp *services.DisclosureResult
	statusErr  error
	calls      []string
}

func (m *mockOversightSvc) OpenDisclosure(_ context.Context, txRef, requestorID, reasonCode string) (*services.DisclosureResult, error) {
	m.calls = append(m.calls, "Open")
	return m.openResp, m.openErr
}
func (m *mockOversightSvc) SignDisclosure(_ context.Context, requestID, signerID string) error {
	m.calls = append(m.calls, "Sign")
	return m.signErr
}
func (m *mockOversightSvc) GetDisclosureStatus(_ context.Context, requestID string) (*services.DisclosureResult, error) {
	m.calls = append(m.calls, "Status")
	return m.statusResp, m.statusErr
}

func newOversightApp(svc OversightServiceIface) *fiber.App {
	app := fiber.New()
	h := NewOversightHandler(svc)
	app.Post("/oversight/disclosure-request", h.OpenDisclosure)
	app.Post("/oversight/disclosure-sign", h.SignDisclosure)
	app.Get("/oversight/disclosure-status/:requestID", h.GetDisclosureStatus)
	return app
}

// (a) OpenDisclosure_Success creates a disclosure request and returns 201.
func TestOversightHandler_OpenDisclosure_Success(t *testing.T) {
	now := time.Now()
	svc := &mockOversightSvc{openResp: &services.DisclosureResult{
		RequestID:            "req-1",
		RequestedByBankID:    "cb_lnet",
		TargetTransactionRef: "0xabc",
		ReasonCode:           "AML_ALERT",
		State:                "PENDING",
		QuorumRequired:       2,
		OpenedAt:             now,
		ExpiresAt:            now.Add(72 * time.Hour),
	}}
	app := newOversightApp(svc)

	body, _ := json.Marshal(map[string]string{
		"tx_ref": "0xabc", "requestor_id": "cb_lnet", "reason_code": "AML_ALERT",
	})
	req := httptest.NewRequest(http.MethodPost, "/oversight/disclosure-request", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	payload, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(payload, []byte("req-1")) {
		t.Error("response must contain request_id")
	}
}

// (b) OpenDisclosure_MissingFields returns 400 when required fields absent.
func TestOversightHandler_OpenDisclosure_MissingFields(t *testing.T) {
	app := newOversightApp(&mockOversightSvc{})

	body, _ := json.Marshal(map[string]string{"tx_ref": "0xabc"})
	req := httptest.NewRequest(http.MethodPost, "/oversight/disclosure-request", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// (c) OpenDisclosure_InvalidReasonCode returns 400 for unrecognised reason_code.
func TestOversightHandler_OpenDisclosure_InvalidReasonCode(t *testing.T) {
	app := newOversightApp(&mockOversightSvc{})

	body, _ := json.Marshal(map[string]string{
		"tx_ref": "0xabc", "requestor_id": "cb_lnet", "reason_code": "UNKNOWN_CODE",
	})
	req := httptest.NewRequest(http.MethodPost, "/oversight/disclosure-request", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid reason_code, got %d", resp.StatusCode)
	}
}

// (d) SignDisclosure_AlreadySigned returns 400 when service signals duplicate.
func TestOversightHandler_SignDisclosure_AlreadySigned(t *testing.T) {
	svc := &mockOversightSvc{signErr: errors.New("signer has already signed this request")}
	app := newOversightApp(svc)

	body, _ := json.Marshal(map[string]string{"request_id": "req-1", "signer_id": "cb_lnet"})
	req := httptest.NewRequest(http.MethodPost, "/oversight/disclosure-sign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// (e) GetDisclosureStatus_NotFound returns 404 when service can't find request.
func TestOversightHandler_GetDisclosureStatus_NotFound(t *testing.T) {
	svc := &mockOversightSvc{statusErr: errors.New("not found")}
	app := newOversightApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/oversight/disclosure-status/missing-id", nil)
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// (f) OpenDisclosure_ServiceError returns 500 when service fails unexpectedly.
func TestOversightHandler_OpenDisclosure_ServiceError(t *testing.T) {
	svc := &mockOversightSvc{openErr: errors.New("db connection lost")}
	app := newOversightApp(svc)

	body, _ := json.Marshal(map[string]string{
		"tx_ref": "0xabc", "requestor_id": "cb_lnet", "reason_code": "AML_ALERT",
	})
	req := httptest.NewRequest(http.MethodPost, "/oversight/disclosure-request", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

// (g) SignDisclosure_Success returns 200 on success.
func TestOversightHandler_SignDisclosure_Success(t *testing.T) {
	svc := &mockOversightSvc{signErr: nil}
	app := newOversightApp(svc)

	body, _ := json.Marshal(map[string]string{"request_id": "req-1", "signer_id": "cb_spoke"})
	req := httptest.NewRequest(http.MethodPost, "/oversight/disclosure-sign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// (h) SignDisclosure_MissingFields returns 400 when required fields are absent.
func TestOversightHandler_SignDisclosure_MissingFields(t *testing.T) {
	app := newOversightApp(&mockOversightSvc{})

	body, _ := json.Marshal(map[string]string{"request_id": "req-1"})
	req := httptest.NewRequest(http.MethodPost, "/oversight/disclosure-sign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing signer_id, got %d", resp.StatusCode)
	}
}

// (i) SignDisclosure_NotFound returns 404 when request doesn't exist.
func TestOversightHandler_SignDisclosure_NotFound(t *testing.T) {
	svc := &mockOversightSvc{signErr: errors.New("disclosure request not found: record not found")}
	app := newOversightApp(svc)

	body, _ := json.Marshal(map[string]string{"request_id": "missing", "signer_id": "cb_lnet"})
	req := httptest.NewRequest(http.MethodPost, "/oversight/disclosure-sign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// (j) SignDisclosure_InternalError returns 500 on unexpected service error.
func TestOversightHandler_SignDisclosure_InternalError(t *testing.T) {
	svc := &mockOversightSvc{signErr: errors.New("unexpected db error")}
	app := newOversightApp(svc)

	body, _ := json.Marshal(map[string]string{"request_id": "req-1", "signer_id": "cb_lnet"})
	req := httptest.NewRequest(http.MethodPost, "/oversight/disclosure-sign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

// (k) GetDisclosureStatus_InternalError returns 500 on unexpected service error.
func TestOversightHandler_GetDisclosureStatus_InternalError(t *testing.T) {
	svc := &mockOversightSvc{statusErr: errors.New("db connection lost")}
	app := newOversightApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/oversight/disclosure-status/req-1", nil)
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

// (l) GetDisclosureStatus_Success returns 200 with disclosure detail.
func TestOversightHandler_GetDisclosureStatus_Success(t *testing.T) {
	now := time.Now()
	closedAt := now.Add(time.Hour)
	svc := &mockOversightSvc{statusResp: &services.DisclosureResult{
		RequestID:            "req-1",
		RequestedByBankID:    "cb_lnet",
		TargetTransactionRef: "0xabc",
		ReasonCode:           "AML_ALERT",
		State:                "QUORUM_REACHED",
		QuorumRequired:       2,
		QuorumReached:        2,
		OpenedAt:             now,
		ExpiresAt:            now.Add(72 * time.Hour),
		ClosedAt:             &closedAt,
	}}
	app := newOversightApp(svc)

	req := httptest.NewRequest(http.MethodGet, "/oversight/disclosure-status/req-1", nil)
	resp, _ := app.Test(req, -1)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	payload, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(payload, []byte("QUORUM_REACHED")) {
		t.Error("response must contain state")
	}
}

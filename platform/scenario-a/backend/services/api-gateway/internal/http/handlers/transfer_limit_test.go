// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	complianceadapter "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/adapters/compliance"
	"github.com/gofiber/fiber/v2"
)

// transferLimitManagerStub satisfies the transferLimitManager interface.
type transferLimitManagerStub struct {
	limits    []complianceadapter.TransferLimit
	createErr error
	listErr   error
	deleteErr error
	deleted   string
}

func (s *transferLimitManagerStub) CreateTransferLimit(_ context.Context, participantID, currency, maxAmount, _ string) (complianceadapter.TransferLimit, error) {
	if s.createErr != nil {
		return complianceadapter.TransferLimit{}, s.createErr
	}
	return complianceadapter.TransferLimit{
		LimitID:       "new-limit-id",
		ParticipantID: participantID,
		Currency:      currency,
		MaxAmount:     maxAmount,
		IsActive:      true,
	}, nil
}

func (s *transferLimitManagerStub) ListTransferLimits(_ context.Context, _ string) ([]complianceadapter.TransferLimit, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.limits, nil
}

func (s *transferLimitManagerStub) DeleteTransferLimit(_ context.Context, limitID, _ string) error {
	s.deleted = limitID
	return s.deleteErr
}

// helper: build a Fiber app with the handler under test, no auth middleware so
// tests can hit handlers directly.
func newTransferLimitApp(stub *transferLimitManagerStub) *fiber.App {
	h := NewTransferLimitHandler(stub)
	app := fiber.New()
	app.Post("/treasury/transfer-limits", h.CreateTransferLimit)
	app.Get("/treasury/transfer-limits", h.ListTransferLimits)
	app.Delete("/treasury/transfer-limits/:id", h.DeleteTransferLimit)
	return app
}

// ── CreateTransferLimit ───────────────────────────────────────────────────────

func TestTransferLimitHandler_Create_Success(t *testing.T) {
	t.Parallel()
	app := newTransferLimitApp(&transferLimitManagerStub{})

	body, _ := json.Marshal(map[string]string{
		"participant_id": "bank-a",
		"currency":       "BRL",
		"max_amount":     "500000",
	})
	req := httptest.NewRequest(http.MethodPost, "/treasury/transfer-limits", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}

	var result complianceadapter.TransferLimit
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.LimitID != "new-limit-id" {
		t.Errorf("limit_id = %q, want %q", result.LimitID, "new-limit-id")
	}
	if result.MaxAmount != "500000" {
		t.Errorf("max_amount = %q, want %q", result.MaxAmount, "500000")
	}
}

func TestTransferLimitHandler_Create_MissingMaxAmount(t *testing.T) {
	t.Parallel()
	app := newTransferLimitApp(&transferLimitManagerStub{})

	body, _ := json.Marshal(map[string]string{"participant_id": "bank-a"})
	req := httptest.NewRequest(http.MethodPost, "/treasury/transfer-limits", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestTransferLimitHandler_Create_AdapterError(t *testing.T) {
	t.Parallel()
	stub := &transferLimitManagerStub{createErr: errors.New("grpc unavailable")}
	app := newTransferLimitApp(stub)

	body, _ := json.Marshal(map[string]string{"max_amount": "1000"})
	req := httptest.NewRequest(http.MethodPost, "/treasury/transfer-limits", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

func TestTransferLimitHandler_Create_InvalidBody(t *testing.T) {
	t.Parallel()
	app := newTransferLimitApp(&transferLimitManagerStub{})

	req := httptest.NewRequest(http.MethodPost, "/treasury/transfer-limits", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// ── ListTransferLimits ────────────────────────────────────────────────────────

func TestTransferLimitHandler_List_Empty(t *testing.T) {
	t.Parallel()
	app := newTransferLimitApp(&transferLimitManagerStub{limits: nil})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/treasury/transfer-limits", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	limits, ok := body["limits"].([]interface{})
	if !ok || len(limits) != 0 {
		t.Errorf("expected empty limits array, got %v", body["limits"])
	}
}

func TestTransferLimitHandler_List_ReturnsList(t *testing.T) {
	t.Parallel()
	stub := &transferLimitManagerStub{
		limits: []complianceadapter.TransferLimit{
			{LimitID: "l1", MaxAmount: "100"},
			{LimitID: "l2", MaxAmount: "200"},
		},
	}
	app := newTransferLimitApp(stub)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/treasury/transfer-limits", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	limits := body["limits"].([]interface{})
	if len(limits) != 2 {
		t.Errorf("expected 2 limits, got %d", len(limits))
	}
}

func TestTransferLimitHandler_List_AdapterError(t *testing.T) {
	t.Parallel()
	app := newTransferLimitApp(&transferLimitManagerStub{listErr: errors.New("grpc down")})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/treasury/transfer-limits", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

// ── DeleteTransferLimit ───────────────────────────────────────────────────────

func TestTransferLimitHandler_Delete_Success(t *testing.T) {
	t.Parallel()
	stub := &transferLimitManagerStub{}
	app := newTransferLimitApp(stub)

	req := httptest.NewRequest(http.MethodDelete, "/treasury/transfer-limits/limit-abc", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
	if stub.deleted != "limit-abc" {
		t.Errorf("expected deleted=%q, got %q", "limit-abc", stub.deleted)
	}
}

func TestTransferLimitHandler_Delete_AdapterError(t *testing.T) {
	t.Parallel()
	app := newTransferLimitApp(&transferLimitManagerStub{deleteErr: errors.New("not found")})

	req := httptest.NewRequest(http.MethodDelete, "/treasury/transfer-limits/bad-id", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

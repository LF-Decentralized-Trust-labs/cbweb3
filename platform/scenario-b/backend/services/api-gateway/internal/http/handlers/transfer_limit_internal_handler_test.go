// SPDX-License-Identifier: Apache-2.0

package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/http/handlers"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"github.com/gofiber/fiber/v2"
)

// stubChecker is an in-memory TransferLimitCheckerIface for handler tests.
type stubChecker struct {
	checkErr     error
	restoreCalls int
}

func (s *stubChecker) CheckAndDeduct(_ context.Context, _, _, _ string) error {
	return s.checkErr
}

func (s *stubChecker) Restore(_ context.Context, _, _, _ string) {
	s.restoreCalls++
}

func setupInternalHandlerApp(checker services.TransferLimitCheckerIface) *fiber.App {
	app := fiber.New()
	h := handlers.NewTransferLimitInternalHandler(checker)
	app.Post("/internal/v2/transfer-limits/check-and-deduct", h.HandleCheckAndDeduct)
	app.Post("/internal/v2/transfer-limits/restore", h.HandleRestore)
	return app
}

func TestHandleCheckAndDeduct_OK(t *testing.T) {
	app := setupInternalHandlerApp(&stubChecker{})
	body, _ := json.Marshal(map[string]string{
		"payer_bank_id": "bank-a",
		"currency":      "BRL",
		"amount_human":  "1000",
	})
	req := httptest.NewRequest("POST", "/internal/v2/transfer-limits/check-and-deduct", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandleCheckAndDeduct_LimitExceeded(t *testing.T) {
	stub := &stubChecker{checkErr: &services.ErrTransferLimitExceeded{PayerBankID: "bank-a", Currency: "BRL", MaxAmount: "500000"}}
	app := setupInternalHandlerApp(stub)
	body, _ := json.Marshal(map[string]string{
		"payer_bank_id": "bank-a",
		"currency":      "BRL",
		"amount_human":  "9999999",
	})
	req := httptest.NewRequest("POST", "/internal/v2/transfer-limits/check-and-deduct", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 422 {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
	respBody, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(respBody), "TRANSFER_LIMIT_EXCEEDED") {
		t.Errorf("expected TRANSFER_LIMIT_EXCEEDED in body, got: %s", string(respBody))
	}
}

func TestHandleCheckAndDeduct_MissingFields(t *testing.T) {
	app := setupInternalHandlerApp(&stubChecker{})
	body, _ := json.Marshal(map[string]string{"currency": "BRL"})
	req := httptest.NewRequest("POST", "/internal/v2/transfer-limits/check-and-deduct", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandleRestore_AlwaysOK(t *testing.T) {
	stub := &stubChecker{}
	app := setupInternalHandlerApp(stub)
	body, _ := json.Marshal(map[string]string{
		"payer_bank_id": "bank-a",
		"currency":      "BRL",
		"amount_human":  "1000",
	})
	req := httptest.NewRequest("POST", "/internal/v2/transfer-limits/restore", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if stub.restoreCalls != 1 {
		t.Fatalf("expected 1 Restore call, got %d", stub.restoreCalls)
	}
}

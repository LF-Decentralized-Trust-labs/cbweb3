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
	return setupInternalHandlerAppAs(checker, "bank-a")
}

// setupInternalHandlerAppAs builds the endpoints as reached by a given verified caller. These routes
// spend (and restore) the named bank's daily allowance, so the caller must be that bank.
func setupInternalHandlerAppAs(checker services.TransferLimitCheckerIface, caller string) *fiber.App {
	app := fiber.New()
	h := handlers.NewTransferLimitInternalHandler(checker)
	app.Post("/internal/v2/transfer-limits/check-and-deduct", asVerifiedCaller(caller), h.HandleCheckAndDeduct)
	app.Post("/internal/v2/transfer-limits/restore", asVerifiedCaller(caller), h.HandleRestore)
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

// Restore credits allowance back, so an unbound caller could undo another bank's consumption and
// with it the limit itself. Check-and-deduct is the mirror image: it could exhaust a competitor's.
func TestTransferLimitInternal_RefusesACallerActingForAnotherBank(t *testing.T) {
	checker := &stubChecker{}
	app := setupInternalHandlerAppAs(checker, "bank-b")

	for _, path := range []string{
		"/internal/v2/transfer-limits/check-and-deduct",
		"/internal/v2/transfer-limits/restore",
	} {
		body, _ := json.Marshal(map[string]string{
			"payer_bank_id": "bank-a",
			"currency":      "BRL",
			"amount_human":  "1000",
		})
		req := httptest.NewRequest("POST", path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 403 {
			t.Fatalf("%s: expected 403 for a caller acting for another bank, got %d", path, resp.StatusCode)
		}
	}
	if checker.restoreCalls != 0 {
		t.Fatalf("no allowance may be restored for a bank the caller is not (restore calls = %d)", checker.restoreCalls)
	}
}

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

	"github.com/gofiber/fiber/v2"
)

// limitCheckerStub implements TransferLimitChecker for handler tests.
type limitCheckerStub struct {
	allowed    bool
	errorCode  string
	maxAmount  string
	checkErr   error
	restoreErr error
	restored   bool
}

func (s *limitCheckerStub) CheckAndDeductTransferLimit(_ context.Context, _, _, _ string) (bool, string, string, error) {
	return s.allowed, s.errorCode, s.maxAmount, s.checkErr
}

func (s *limitCheckerStub) RestoreTransferLimit(_ context.Context, _, _, _ string) error {
	s.restored = true
	return s.restoreErr
}

// newLimitCheckApp wires a minimal Fiber route that exercises checkAndDeductLimit
// without needing a real payment gRPC connection.
func newLimitCheckApp(checker TransferLimitChecker) *fiber.App {
	h := &PaymentHandler{
		bankCode:     "bank-a",
		fiatSymbol:   "BRL",
		limitChecker: checker,
	}
	app := fiber.New()
	app.Post("/check", func(c *fiber.Ctx) error {
		if ok, err := h.checkAndDeductLimit(c, "1000"); !ok {
			return err // nil; response already written (422/503)
		}
		return c.SendStatus(fiber.StatusOK)
	})
	return app
}

// ── checkAndDeductLimit helper ────────────────────────────────────────────────

func TestCheckAndDeductLimit_NilChecker_Passthrough(t *testing.T) {
	t.Parallel()
	h := &PaymentHandler{bankCode: "bank-a", limitChecker: nil, fiatSymbol: "BRL"}
	app := fiber.New()
	app.Post("/check", func(c *fiber.Ctx) error {
		if ok, err := h.checkAndDeductLimit(c, "9999"); !ok {
			return err
		}
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/check", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 (passthrough without checker), got %d", resp.StatusCode)
	}
}

func TestCheckAndDeductLimit_EmptyFiatSymbol_Passthrough(t *testing.T) {
	t.Parallel()
	// Checker is set but fiatSymbol is empty → should behave as no-op.
	h := &PaymentHandler{
		bankCode:     "bank-a",
		fiatSymbol:   "",
		limitChecker: &limitCheckerStub{allowed: false, errorCode: "TRANSFER_LIMIT_EXCEEDED"},
	}
	app := fiber.New()
	app.Post("/check", func(c *fiber.Ctx) error {
		if ok, err := h.checkAndDeductLimit(c, "9999"); !ok {
			return err
		}
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/check", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 (fiatSymbol empty = no-op), got %d", resp.StatusCode)
	}
}

func TestCheckAndDeductLimit_Allowed(t *testing.T) {
	t.Parallel()
	checker := &limitCheckerStub{allowed: true}
	resp, err := newLimitCheckApp(checker).Test(httptest.NewRequest(http.MethodPost, "/check", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestCheckAndDeductLimit_Exceeded_Returns422(t *testing.T) {
	t.Parallel()
	checker := &limitCheckerStub{allowed: false, errorCode: "TRANSFER_LIMIT_EXCEEDED", maxAmount: "500"}
	resp, err := newLimitCheckApp(checker).Test(httptest.NewRequest(http.MethodPost, "/check", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}

	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["error_code"] != "TRANSFER_LIMIT_EXCEEDED" {
		t.Errorf("error_code = %q, want TRANSFER_LIMIT_EXCEEDED", body["error_code"])
	}
	if body["recommended_action"] == "" {
		t.Error("expected recommended_action in response body")
	}
}

func TestCheckAndDeductLimit_ServiceUnavailable_Returns503(t *testing.T) {
	t.Parallel()
	checker := &limitCheckerStub{checkErr: errors.New("connection refused")}
	resp, err := newLimitCheckApp(checker).Test(httptest.NewRequest(http.MethodPost, "/check", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}

	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["error_code"] != "LIMIT_SERVICE_UNAVAILABLE" {
		t.Errorf("error_code = %q, want LIMIT_SERVICE_UNAVAILABLE", body["error_code"])
	}
}

// ── restoreLimit helper ───────────────────────────────────────────────────────

func TestRestoreLimit_CallsCheckerOnError(t *testing.T) {
	t.Parallel()
	checker := &limitCheckerStub{allowed: true}
	h := &PaymentHandler{bankCode: "bank-a", fiatSymbol: "BRL", limitChecker: checker}

	// restoreLimit is called via a Fiber context; we simulate it with a minimal app.
	app := fiber.New()
	app.Post("/restore", func(c *fiber.Ctx) error {
		h.restoreLimit(c, "500")
		return c.SendStatus(fiber.StatusOK)
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/restore", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = resp
	if !checker.restored {
		t.Error("expected restoreLimit to call checker.RestoreTransferLimit")
	}
}

func TestRestoreLimit_NilChecker_NoOp(t *testing.T) {
	t.Parallel()
	h := &PaymentHandler{bankCode: "bank-a", fiatSymbol: "BRL", limitChecker: nil}
	app := fiber.New()
	app.Post("/restore", func(c *fiber.Ctx) error {
		h.restoreLimit(c, "500") // must not panic
		return c.SendStatus(fiber.StatusOK)
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/restore", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// ── LockHTLC — limit enforcement via full handler body ────────────────────────

// htlcLimitTestApp builds a Fiber app that wires LockHTLC with a real (stub)
// limit checker and a nil payment adapter; because the limit is exceeded the
// handler returns before ever touching h.payment.
func htlcLimitTestApp(checker TransferLimitChecker) *fiber.App {
	h := &PaymentHandler{
		payment:      nil, // never reached when limit exceeded
		bankCode:     "bank-a",
		fiatSymbol:   "BRL",
		limitChecker: checker,
	}
	app := fiber.New()
	app.Post("/htlc/lock", h.LockHTLC)
	app.Post("/htlc/lock-with-hash", h.LockHTLCWithHashLock)
	return app
}

func TestLockHTLC_LimitExceeded_Returns422(t *testing.T) {
	t.Parallel()
	checker := &limitCheckerStub{allowed: false, errorCode: "TRANSFER_LIMIT_EXCEEDED"}
	app := htlcLimitTestApp(checker)

	body, _ := json.Marshal(map[string]interface{}{
		"receiver": "bank-b",
		"amount":   "1000",
	})
	req := httptest.NewRequest(http.MethodPost, "/htlc/lock", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 (limit exceeded), got %d", resp.StatusCode)
	}
}

func TestLockHTLCWithHashLock_LimitExceeded_Returns422(t *testing.T) {
	t.Parallel()
	checker := &limitCheckerStub{allowed: false, errorCode: "TRANSFER_LIMIT_EXCEEDED"}
	app := htlcLimitTestApp(checker)

	body, _ := json.Marshal(map[string]interface{}{
		"receiver":  "bank-b",
		"amount":    "500",
		"hash_lock": "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
	})
	req := httptest.NewRequest(http.MethodPost, "/htlc/lock-with-hash", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 (limit exceeded), got %d", resp.StatusCode)
	}
}

func TestLockHTLC_MissingRequiredFields_400(t *testing.T) {
	t.Parallel()
	// Missing receiver/amount → 400 before limit check is reached.
	checker := &limitCheckerStub{allowed: true}
	app := htlcLimitTestApp(checker)

	body, _ := json.Marshal(map[string]interface{}{})
	req := httptest.NewRequest(http.MethodPost, "/htlc/lock", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing fields, got %d", resp.StatusCode)
	}
}

// ── WithLimitChecker ──────────────────────────────────────────────────────────

func TestWithLimitChecker_SetsFields(t *testing.T) {
	t.Parallel()
	checker := &limitCheckerStub{allowed: true}
	h := NewPaymentHandler(nil, "bank-a")
	h2 := h.WithLimitChecker(checker, "BRL")

	// WithLimitChecker returns the same pointer (mutates in place).
	if h2 != h {
		t.Error("WithLimitChecker should return the same *PaymentHandler")
	}
	if h.limitChecker != checker {
		t.Error("limitChecker field was not set")
	}
	if h.fiatSymbol != "BRL" {
		t.Errorf("fiatSymbol = %q, want BRL", h.fiatSymbol)
	}
}

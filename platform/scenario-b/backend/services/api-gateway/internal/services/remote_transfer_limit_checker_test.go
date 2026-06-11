package services_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
)

func TestRemoteTransferLimitChecker_CheckAndDeduct_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/v2/transfer-limits/check-and-deduct" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-Relay-Auth") != "testsecret" {
			t.Error("missing or wrong X-Relay-Auth header")
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer srv.Close()

	checker := services.NewRemoteTransferLimitChecker(srv.URL, "testsecret", 5*time.Second)
	if err := checker.CheckAndDeduct(context.Background(), "bank-a", "BRL", "1000"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestRemoteTransferLimitChecker_CheckAndDeduct_LimitExceeded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{
			"error_code": "TRANSFER_LIMIT_EXCEEDED",
			"error":      "daily transfer limit exceeded",
		})
	}))
	defer srv.Close()

	checker := services.NewRemoteTransferLimitChecker(srv.URL, "secret", 5*time.Second)
	err := checker.CheckAndDeduct(context.Background(), "bank-a", "BRL", "9999999")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var limitErr *services.ErrTransferLimitExceeded
	if !isErrTransferLimitExceeded(err, &limitErr) {
		t.Fatalf("expected *ErrTransferLimitExceeded, got %T: %v", err, err)
	}
}

func TestRemoteTransferLimitChecker_CheckAndDeduct_CBUnavailable(t *testing.T) {
	checker := services.NewRemoteTransferLimitChecker("http://127.0.0.1:0", "secret", 100*time.Millisecond)
	err := checker.CheckAndDeduct(context.Background(), "bank-a", "BRL", "100")
	if err == nil {
		t.Fatal("expected error when CB is unreachable")
	}
}

func TestRemoteTransferLimitChecker_Restore_BestEffort(t *testing.T) {
	// Restore must not panic even when CB returns error or is unreachable.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	checker := services.NewRemoteTransferLimitChecker(srv.URL, "secret", 5*time.Second)
	// Must not panic or return anything.
	checker.Restore(context.Background(), "bank-a", "BRL", "100")
}

// isErrTransferLimitExceeded checks if err is *ErrTransferLimitExceeded using errors.As semantics.
func isErrTransferLimitExceeded(err error, target **services.ErrTransferLimitExceeded) bool {
	if e, ok := err.(*services.ErrTransferLimitExceeded); ok {
		*target = e
		return true
	}
	return false
}

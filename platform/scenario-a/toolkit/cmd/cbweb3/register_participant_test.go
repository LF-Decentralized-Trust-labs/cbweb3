// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLookupBankWallet_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/onboarding/my-status" || r.URL.Query().Get("bank_code") != "bank-itau" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"request_id":"r1","user_id":"u1","status":"KYC_APPROVED","wallet_address":"0xAbC123"}`))
	}))
	defer srv.Close()

	got, err := lookupBankWallet(context.Background(), srv.URL, "bank-itau")
	if err != nil {
		t.Fatalf("lookupBankWallet: %v", err)
	}
	if got != "0xAbC123" {
		t.Errorf("wallet = %q; want 0xAbC123", got)
	}
}

func TestLookupBankWallet_NoWallet(t *testing.T) {
	// Bank that has not completed onboarding yet: my-status has no wallet_address.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"request_id":"r1","user_id":"u1","status":"CREDENTIAL_REQUESTED"}`))
	}))
	defer srv.Close()

	_, err := lookupBankWallet(context.Background(), srv.URL, "bank-itau")
	if err == nil || !strings.Contains(err.Error(), "no wallet_address") {
		t.Errorf("expected no-wallet error, got %v", err)
	}
}

func TestLookupBankWallet_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := lookupBankWallet(context.Background(), srv.URL, "bank-itau")
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("expected HTTP 404 error, got %v", err)
	}
}

func TestRunRegisterParticipant_FlagValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"missing -f", []string{"--bank", "bank-itau"}},
		{"missing --bank", []string{"-f", "testdata/central-bank-brl.yaml"}},
		{"missing manifest file", []string{"-f", "testdata/does-not-exist.yaml", "--bank", "bank-itau"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if code := runRegisterParticipant(tc.args); code != 1 {
				t.Errorf("exit = %d; want 1", code)
			}
		})
	}
}

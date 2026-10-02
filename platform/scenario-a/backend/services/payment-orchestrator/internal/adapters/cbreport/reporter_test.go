// SPDX-License-Identifier: Apache-2.0

package cbreport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

func TestReporter_ReportSettledLeg_PostsLegWithAuth(t *testing.T) {
	var gotPath, gotAuth, gotCT string
	var gotBody legPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("X-Relay-Auth")
		gotCT = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	r := New(srv.URL, "s3cr3t")
	leg := ports.SettledLeg{
		TradeID:    "trade-1",
		ContractID: "c1",
		Sender:     "op@spoke-a-bank-a",
		Receiver:   "corr@spoke-a-bank-c",
		Amount:     "700",
		SettledAt:  time.Date(2026, 7, 11, 10, 0, 0, 0, time.UTC),
	}
	if err := r.ReportSettledLeg(context.Background(), leg); err != nil {
		t.Fatalf("ReportSettledLeg: %v", err)
	}

	if gotPath != "/internal/v1/payments/pvp-legs" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAuth != "s3cr3t" {
		t.Errorf("X-Relay-Auth = %q, want s3cr3t", gotAuth)
	}
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q", gotCT)
	}
	if gotBody.TradeID != "trade-1" || gotBody.ContractID != "c1" || gotBody.Receiver != "corr@spoke-a-bank-c" || gotBody.Amount != "700" {
		t.Errorf("unexpected body: %+v", gotBody)
	}
	if gotBody.SettledAt != "2026-07-11T10:00:00Z" {
		t.Errorf("settled_at = %q, want RFC3339 UTC", gotBody.SettledAt)
	}
}

func TestReporter_ReportSettledLeg_ErrorsOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	// noSleep: a 500 is now retried, so without it this case pays the real backoff
	// and this test alone took six seconds.
	r := New(srv.URL, "")
	r.noSleep()
	if err := r.ReportSettledLeg(context.Background(), ports.SettledLeg{ContractID: "c1"}); err == nil {
		t.Fatal("expected error on 500, got nil")
	}
}

// TestReporter_RetriesTransientFailures pins the bounded retry. The report is the only
// record of an incoming leg, so losing it on the first hiccup silently removes a
// settled movement from the ledger.
func TestReporter_RetriesTransientFailures(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusBadGateway) // 5xx — worth another attempt
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	r := New(srv.URL, "")
	r.noSleep()
	if err := r.ReportSettledLeg(context.Background(), ports.SettledLeg{ContractID: "c1"}); err != nil {
		t.Fatalf("a leg that succeeds on the third attempt must be reported: %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

// TestReporter_DoesNotRetryA4xx guards against hammering the central bank with a
// payload it has already rejected. Repeating the same bytes cannot change a 4xx.
func TestReporter_DoesNotRetryA4xx(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	r := New(srv.URL, "")
	r.noSleep()
	if err := r.ReportSettledLeg(context.Background(), ports.SettledLeg{ContractID: "c1"}); err == nil {
		t.Fatal("a 4xx must be reported as an error")
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 — a 4xx must not be retried", attempts)
	}
}

// TestReporter_GivesUpAfterMaxAttempts bounds the loop: an unavailable central bank
// must not be retried forever.
func TestReporter_GivesUpAfterMaxAttempts(t *testing.T) {
	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	r := New(srv.URL, "")
	r.noSleep()
	if err := r.ReportSettledLeg(context.Background(), ports.SettledLeg{ContractID: "c1"}); err == nil {
		t.Fatal("an exhausted retry budget must surface as an error, not silence")
	}
	if attempts != maxAttempts {
		t.Errorf("attempts = %d, want %d", attempts, maxAttempts)
	}
}

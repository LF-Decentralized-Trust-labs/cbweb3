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

	if err := New(srv.URL, "").ReportSettledLeg(context.Background(), ports.SettledLeg{ContractID: "c1"}); err == nil {
		t.Fatal("expected error on 500, got nil")
	}
}

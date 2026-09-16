// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"testing"
	"time"

	authz "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/authz"
	compliancv1 "github.com/LACNetNetworks/cbweb3-platform/backend/shared/proto/compliance/v1"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Characterisation of the halt control: a governance flag recorded in the database,
// which is the only implementation there is.
//
// These tests were written before the on-chain branches were removed, and they passed
// unmodified afterwards. That is what makes "the halt control is unchanged" a measured
// claim rather than an assurance — they never referenced the breaker field, so the
// removal could not have quietly weakened them.

// TestCircuitBreakerDBOnly_PersistsStateUpdaterAndTimestamp pins what a toggle
// actually writes: the three system parameters, the actor taken from the request
// context, and an ISO-8601 timestamp.
func TestCircuitBreakerDBOnly_PersistsStateUpdaterAndTimestamp(t *testing.T) {
	t.Parallel()
	svc := newTestService()
	ctx := authz.NewContext(context.Background(), &authz.Identity{Subject: "noc-operator", Method: "mtls"})

	if _, err := svc.ToggleCircuitBreaker(ctx, &compliancv1.ToggleCircuitBreakerRequest{Pause: true, Reason: "incident"}); err != nil {
		t.Fatalf("pause: %v", err)
	}

	paused, ok, err := svc.repo.GetSystemParameter(ctx, paramCircuitBreakerPaused)
	if err != nil || !ok {
		t.Fatalf("reading %s: ok=%v err=%v", paramCircuitBreakerPaused, ok, err)
	}
	if paused != "true" {
		t.Errorf("%s = %q, want \"true\"", paramCircuitBreakerPaused, paused)
	}

	updatedBy, ok, err := svc.repo.GetSystemParameter(ctx, paramCircuitBreakerUpdatedBy)
	if err != nil || !ok {
		t.Fatalf("reading %s: ok=%v err=%v", paramCircuitBreakerUpdatedBy, ok, err)
	}
	if updatedBy != "noc-operator" {
		t.Errorf("%s = %q, want the request's actor", paramCircuitBreakerUpdatedBy, updatedBy)
	}

	updatedAt, ok, err := svc.repo.GetSystemParameter(ctx, paramCircuitBreakerUpdatedAt)
	if err != nil || !ok {
		t.Fatalf("reading %s: ok=%v err=%v", paramCircuitBreakerUpdatedAt, ok, err)
	}
	if _, err := time.Parse(time.RFC3339, updatedAt); err != nil {
		t.Errorf("%s = %q, not RFC3339: %v", paramCircuitBreakerUpdatedAt, updatedAt, err)
	}

	// Resuming mirrors the requested state rather than leaving the pause standing.
	if _, err := svc.ToggleCircuitBreaker(ctx, &compliancv1.ToggleCircuitBreakerRequest{Pause: false, Reason: "resolved"}); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if paused, _, _ := svc.repo.GetSystemParameter(ctx, paramCircuitBreakerPaused); paused != "false" {
		t.Errorf("%s after resume = %q, want \"false\"", paramCircuitBreakerPaused, paused)
	}
}

// TestCircuitBreakerDBOnly_EmptyTxHashIsNotAnError is the rule that matters most
// after the retirement: with no chain in the path there is no transaction to quote,
// and the absence must not read as a failure to any caller. A future change that
// starts erroring, or that invents a placeholder hash, fails here.
func TestCircuitBreakerDBOnly_EmptyTxHashIsNotAnError(t *testing.T) {
	t.Parallel()
	svc := newTestService()
	ctx := authz.NewContext(context.Background(), &authz.Identity{Subject: "noc-operator", Method: "mtls"})

	for _, tc := range []struct {
		name  string
		pause bool
	}{
		{"pause", true},
		{"resume", false},
	} {
		resp, err := svc.ToggleCircuitBreaker(ctx, &compliancv1.ToggleCircuitBreakerRequest{Pause: tc.pause, Reason: "characterisation"})
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if resp.TxHash != "" {
			t.Errorf("%s: TxHash = %q, want empty — there is no chain in this path", tc.name, resp.TxHash)
		}
		if resp.IsPaused != tc.pause {
			t.Errorf("%s: IsPaused = %v, want %v", tc.name, resp.IsPaused, tc.pause)
		}
	}
}

// TestGetCircuitBreakerStatusDBOnly_ReportsRecordedState covers the read side,
// including the never-toggled case: an unset breaker reports "not paused" with an
// empty updater and timestamp, and that is a successful response, not an error.
func TestGetCircuitBreakerStatusDBOnly_ReportsRecordedState(t *testing.T) {
	t.Parallel()
	svc := newTestService()
	ctx := authz.NewContext(context.Background(), &authz.Identity{Subject: "noc-operator", Method: "mtls"})

	st, err := svc.GetCircuitBreakerStatus(ctx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("status with nothing recorded: %v", err)
	}
	if st.IsPaused {
		t.Error("expected not paused when nothing was ever recorded")
	}
	if st.UpdatedBy != "" || st.LastUpdate != "" {
		t.Errorf("expected empty updater and timestamp, got UpdatedBy=%q LastUpdate=%q", st.UpdatedBy, st.LastUpdate)
	}

	if _, err := svc.ToggleCircuitBreaker(ctx, &compliancv1.ToggleCircuitBreakerRequest{Pause: true, Reason: "incident"}); err != nil {
		t.Fatalf("pause: %v", err)
	}

	st2, err := svc.GetCircuitBreakerStatus(ctx, &emptypb.Empty{})
	if err != nil {
		t.Fatalf("status after pause: %v", err)
	}
	if !st2.IsPaused {
		t.Error("expected the recorded pause to be reported")
	}
	if st2.UpdatedBy != "noc-operator" {
		t.Errorf("UpdatedBy = %q, want the actor that toggled", st2.UpdatedBy)
	}
	if _, err := time.Parse(time.RFC3339, st2.LastUpdate); err != nil {
		t.Errorf("LastUpdate = %q, not RFC3339: %v", st2.LastUpdate, err)
	}
}

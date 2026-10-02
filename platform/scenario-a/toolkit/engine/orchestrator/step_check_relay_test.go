// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A bank settles its own HTLC leg — transferLocked runs on its own Paladin node — and it
// learns a leg needs settling from the relay's journal, which it polls. An endpoint that is
// merely SET is not enough: the LNET shape is a bank on its own VM inheriting a co-located
// default that resolves to nothing there. The bank joins clean and then never settles, with
// nothing saying why. This step is the only place in the join that finds that out.
func TestCheckRelayStep_FailsWhenRelayIsUnreachable(t *testing.T) {
	// A port nothing listens on. Closing immediately gives us one that is free.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	dead := srv.URL
	srv.Close()

	step := newCheckRelayStep(dead, 300*time.Millisecond, 50*time.Millisecond, io.Discard)
	err := step.Run(context.Background())
	if err == nil {
		t.Fatal("check-relay must fail when the relay cannot be reached")
	}
	if !strings.Contains(err.Error(), dead) {
		t.Errorf("the error must name the endpoint that failed so an operator can see which one is wrong, got: %v", err)
	}
}

func TestCheckRelayStep_PassesWhenRelayAnswers(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	step := newCheckRelayStep(srv.URL, 2*time.Second, 50*time.Millisecond, io.Discard)
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("check-relay must pass against a live relay: %v", err)
	}
	if path != "/api/v1/health" {
		t.Errorf("probed %q; want the relay's health endpoint /api/v1/health", path)
	}
}

// A relay that is up but answering 5xx is not usable either, and saying "unreachable" would
// send an operator to the network when the problem is the relay itself.
func TestCheckRelayStep_FailsAndReportsStatusOnServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	step := newCheckRelayStep(srv.URL, 300*time.Millisecond, 50*time.Millisecond, io.Discard)
	err := step.Run(context.Background())
	if err == nil {
		t.Fatal("check-relay must fail when the relay answers an error status")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("the error must carry the status the relay returned, got: %v", err)
	}
}

// It retries: a relay still starting up must not fail the join on the first attempt.
func TestCheckRelayStep_RetriesUntilTheRelayIsUp(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	step := newCheckRelayStep(srv.URL, 3*time.Second, 20*time.Millisecond, io.Discard)
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("check-relay must keep trying while the relay is still coming up: %v", err)
	}
	if hits < 3 {
		t.Errorf("expected at least 3 attempts, got %d", hits)
	}
}

func TestCheckRelayStep_Name(t *testing.T) {
	if got := newCheckRelayStep("http://x", time.Second, time.Second, io.Discard).Name(); got != StepCheckRelay {
		t.Errorf("Name() = %q; want %q", got, StepCheckRelay)
	}
}

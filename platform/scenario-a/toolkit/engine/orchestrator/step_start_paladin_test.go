// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStartPaladinStep_Check_False_DockerNotAvailable(t *testing.T) {
	dir := t.TempDir()
	// nonexistent compose file → docker compose ps fails → containerRunning = false
	step := newStartPaladinStep("spoke-test", dir, "/nonexistent/paladin-compose.yaml",
		"http://localhost:31648", "", 5*time.Second, 500*time.Millisecond)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should return false when docker compose ps fails")
	}
}

func TestStartPaladinStep_PaladinHealthy_True(t *testing.T) {
	// Server that responds with PD020704 error code — signals Paladin is up.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"jsonrpc":"2.0","id":1,"error":{"code":"PD020704","message":"not found"}}`)
	}))
	defer srv.Close()

	dir := t.TempDir()
	s := &startPaladinStep{
		spokeID:        "spoke-test",
		dataDir:        dir,
		composePath:    "/nonexistent/compose.yaml",
		paladinCBURL:   srv.URL,
		healthTimeout:  5 * time.Second,
		healthInterval: 100 * time.Millisecond,
	}
	if !s.paladinHealthy(context.Background()) {
		t.Error("paladinHealthy should return true when response contains PD020704")
	}
}

func TestStartPaladinStep_PaladinHealthy_False_NoServer(t *testing.T) {
	dir := t.TempDir()
	s := &startPaladinStep{
		spokeID:        "spoke-test",
		dataDir:        dir,
		paladinCBURL:   "http://127.0.0.1:1", // nothing listening
		healthTimeout:  1 * time.Second,
		healthInterval: 100 * time.Millisecond,
	}
	if s.paladinHealthy(context.Background()) {
		t.Error("paladinHealthy should return false when no server is reachable")
	}
}

func TestStartPaladinStep_PaladinHealthy_False_WrongResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"jsonrpc":"2.0","id":1,"result":null}`)
	}))
	defer srv.Close()

	dir := t.TempDir()
	s := &startPaladinStep{
		spokeID:      "spoke-test",
		dataDir:      dir,
		paladinCBURL: srv.URL,
	}
	if s.paladinHealthy(context.Background()) {
		t.Error("paladinHealthy should return false when PD020704 is not in response")
	}
}

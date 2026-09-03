// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStartPaladinStep_Check_False_DockerNotAvailable(t *testing.T) {
	dir := t.TempDir()
	// nonexistent compose file → docker compose ps fails → containerRunning = false
	step := newStartPaladinStep("spoke-test", dir, "/nonexistent/paladin-compose.yaml",
		"http://localhost:31648", "", "", 5*time.Second, 500*time.Millisecond)
	done, err := step.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if done {
		t.Error("check should return false when docker compose ps fails")
	}
}

func TestIsRoutableHost(t *testing.T) {
	for _, tc := range []struct {
		host string
		want bool
	}{
		{"", false},
		{"localhost", false},
		{"127.0.0.1", false},
		{"::1", false},
		{"host.docker.internal", false},
		{"host-gateway", false},
		{"  LOCALHOST  ", false},
		{"10.10.0.21", true},
		{"cb-brazil.cbweb3.l-net.io", true},
	} {
		if got := isRoutableHost(tc.host); got != tc.want {
			t.Errorf("isRoutableHost(%q) = %v; want %v", tc.host, got, tc.want)
		}
	}
}

func TestStartPaladinStep_ComposeEnv_GRPCPort(t *testing.T) {
	// Single-host (non-routable advertised host): gRPC stays in the rpcPort+2 band.
	local := &startPaladinStep{spokeID: "spoke-brl", dataDir: t.TempDir(),
		paladinCBURL: "http://localhost:31648", advertisedHost: "host.docker.internal"}
	if env := strings.Join(local.composeEnv(), "\n"); !strings.Contains(env, "PALADIN_CB_GRPC_PORT=31650") {
		t.Errorf("single-host: want PALADIN_CB_GRPC_PORT=31650 (rpc+2), got:\n%s", env)
	}
	// Routable CB (multi-VM): gRPC is exposed on the fixed peer port 9000 so joining
	// banks can dial the on-chain dns:///paladin-<spoke>-cb:9000 endpoint.
	routable := &startPaladinStep{spokeID: "spoke-brl", dataDir: t.TempDir(),
		paladinCBURL: "http://localhost:31648", advertisedHost: "10.10.0.21"}
	if env := strings.Join(routable.composeEnv(), "\n"); !strings.Contains(env, "PALADIN_CB_GRPC_PORT=9000") {
		t.Errorf("routable: want PALADIN_CB_GRPC_PORT=9000, got:\n%s", env)
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

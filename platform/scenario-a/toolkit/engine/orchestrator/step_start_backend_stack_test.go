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

func TestStartBackendStackStep_ComposeEnv(t *testing.T) {
	s := newStartBackendStackStep("start-cb-backend", backendStackParams{
		EntityPrefix:    "cbweb3-central-bank-brazil",
		NetName:         "cbweb3-central-bank-brazil-net",
		BackendContext:  "/repo/scenario-a/backend",
		EnvFile:         "/data/central-bank-brazil/.env.infra",
		BankCode:        "central-bank-brazil",
		PaladinURL:      "http://host.docker.internal:31648",
		PaladinIdentity: "funded_operator@spoke-brl-cb",
		APIPort:         18645,
		AuthPort:        19645,
		CompliancePort:  20645,
		PaymentPort:     21645,
	}).(*startBackendStackStep)

	env := strings.Join(s.composeEnv(), "\n")
	for _, want := range []string{
		"ENTITY_PREFIX=cbweb3-central-bank-brazil",
		"BACKEND_CONTEXT=/repo/scenario-a/backend",
		"ENTITY_ENV_FILE=/data/central-bank-brazil/.env.infra",
		"API_GATEWAY_PORT=18645",
		"PALADIN_IDENTITY=funded_operator@spoke-brl-cb",
		"CACTI_API_URL=http://host.docker.internal:4000", // default when CactiURL is empty
		"BACKEND_IMAGE_TAG=local",                        // default
	} {
		if !strings.Contains(env, want) {
			t.Errorf("composeEnv missing %q", want)
		}
	}
}

// TestStartBackendStackStep_ComposeEnv_UseTLSVolume verifies the CB path: when
// UseTLSVolume is set (instead of PKIDir), ENTITY_PKI_DIR resolves to the fixed
// compose-local volume key (entityPKIVolumeKey), not a host path — the actual
// per-spoke volume name comes from backend-compose.yaml's own
// name: ${SPOKE_ID}_cb_tls, which is why SPOKE_ID must also be set.
func TestStartBackendStackStep_ComposeEnv_UseTLSVolume(t *testing.T) {
	s := newStartBackendStackStep("start-cb-backend", backendStackParams{
		SpokeID:        "spoke-brl",
		EntityPrefix:   "cbweb3-central-bank-brazil",
		NetName:        "cbweb3-central-bank-brazil-net",
		BackendContext: "/repo/scenario-a/backend",
		UseTLSVolume:   true,
		APIPort:        18645,
	}).(*startBackendStackStep)

	env := strings.Join(s.composeEnv(), "\n")
	for _, want := range []string{
		"SPOKE_ID=spoke-brl",
		"ENTITY_PKI_DIR=" + entityPKIVolumeKey,
	} {
		if !strings.Contains(env, want) {
			t.Errorf("composeEnv missing %q, got:\n%s", want, env)
		}
	}
}

// TestCactiContainerURL verifies that cactiContainerURL produces the correct
// CACTI_API_URL for each deployment topology:
//   - empty endpoint   → co-located default (host.docker.internal:4000)
//   - localhost URL    → rewritten to host.docker.internal (relay co-located, different port)
//   - external IP URL  → used as-is (multi-host relay deployment)
func TestCactiContainerURL(t *testing.T) {
	cases := []struct {
		endpoint string
		want     string
	}{
		// Empty: relay is co-located but endpoint not explicitly set.
		{"", "http://host.docker.internal:4000"},
		// localhost: relay co-located; rewrite so containers can reach the host-published port.
		{"http://localhost:4000", "http://host.docker.internal:4000"},
		// External IP: multi-host deployment — use as-is so containers reach the remote relay.
		{"http://18.208.191.195:4000", "http://18.208.191.195:4000"},
		// External hostname (e.g. DNS-resolved relay in staging).
		{"http://relay.cbweb3.example.com:4000", "http://relay.cbweb3.example.com:4000"},
	}
	for _, tc := range cases {
		got := cactiContainerURL(tc.endpoint)
		if got != tc.want {
			t.Errorf("cactiContainerURL(%q) = %q, want %q", tc.endpoint, got, tc.want)
		}
	}
}

// TestStartBackendStackStep_ComposeEnv_ExternalRelay checks that a non-empty
// CactiURL (set by buildSteps/buildJoinSteps from relay.endpoint) is honoured
// over the host.docker.internal default.
func TestStartBackendStackStep_ComposeEnv_ExternalRelay(t *testing.T) {
	s := newStartBackendStackStep("start-cb-backend", backendStackParams{
		EntityPrefix: "cbweb3-central-bank-brazil",
		NetName:      "cbweb3-central-bank-brazil-net",
		BankCode:     "central-bank-brazil",
		CactiURL:     "http://18.208.191.195:4000",
		APIPort:      18645,
		AuthPort:     19645,
		CompliancePort: 20645,
		PaymentPort:  21645,
	}).(*startBackendStackStep)

	env := strings.Join(s.composeEnv(), "\n")
	if !strings.Contains(env, "CACTI_API_URL=http://18.208.191.195:4000") {
		t.Errorf("composeEnv should contain external relay URL, got:\n%s", env)
	}
	if strings.Contains(env, "host.docker.internal") {
		t.Errorf("composeEnv should NOT contain host.docker.internal when external relay is set, got:\n%s", env)
	}
}

func TestStartBackendStackStep_Check_HealthEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	// httpHealthy must report the live server healthy and a dead port not.
	if !httpHealthy(context.Background(), srv.URL+"/api/v1/health") {
		t.Error("expected healthy for 200 server")
	}
	if httpHealthy(context.Background(), fmt.Sprintf("http://localhost:%d/api/v1/health", 1)) {
		t.Error("expected unhealthy for closed port")
	}
}

func TestStartBackendStackStep_Run_TimesOutWithoutBackend(t *testing.T) {
	// Compose path nonexistent → compose up fails fast (no health wait).
	s := newStartBackendStackStep("start-cb-backend", backendStackParams{
		ComposePath:    "/nonexistent/backend-compose.yaml",
		APIPort:        18645,
		HealthTimeout:  100 * time.Millisecond,
		HealthInterval: 50 * time.Millisecond,
	})
	if err := s.Run(context.Background()); err == nil {
		t.Error("Run should error when compose file is missing")
	}
}

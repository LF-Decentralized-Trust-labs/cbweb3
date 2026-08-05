// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// observeProxyConfig is a routable, proxy-enabled observe config (TLS host ⇒ https origin).
func observeProxyConfig() ObserveConfig {
	c := ObserveConfig{
		ContainerPrefix: "sc-b-cbweb3-noc-brazil",
		NetPrefix:       "noc-brazil",
		VolumePrefix:    "noc-brazil",
		FrontendHost:    "cb-brazil.example",
		KeycloakURL:     "https://kc.example/auth",
		LauncherURL:     "https://cb-brazil.example/",
		ProxyEnabled:    true,
	}
	c.WithDefaults()
	return c
}

// REGRESSION LOCK: with proxy disabled the portal build args must stay byte-for-byte the
// port-based local default — no VITE_BASE_PATH, backend on the published host port. This
// is the "local & non-proxy deployments unchanged" contract; do not relax it.
func TestObservePortalViteArgs_NonProxy_Frozen(t *testing.T) {
	c := ObserveConfig{FrontendHost: "localhost", KeycloakURL: "http://localhost:8081"}
	c.WithDefaults() // BackendPort → 8090
	args := c.portalViteArgs()

	if got, want := args["VITE_NOC_BACKEND_URL"], "http://localhost:8090/api/v1"; got != want {
		t.Errorf("VITE_NOC_BACKEND_URL = %q, want %q", got, want)
	}
	if _, ok := args["VITE_BASE_PATH"]; ok {
		t.Errorf("VITE_BASE_PATH must be absent in non-proxy mode; got %q", args["VITE_BASE_PATH"])
	}
	if got, want := args["VITE_KEYCLOAK_CLIENT_ID"], nocKeycloakClient; got != want {
		t.Errorf("VITE_KEYCLOAK_CLIENT_ID = %q, want %q", got, want)
	}
}

func TestObservePortalViteArgs_Proxy(t *testing.T) {
	t.Setenv("PROXY_TLS_MODE", "internal") // routable host ⇒ https origin
	c := observeProxyConfig()
	args := c.portalViteArgs()

	if got, want := args["VITE_BASE_PATH"], "/b/noc/"; got != want {
		t.Errorf("VITE_BASE_PATH = %q, want %q", got, want)
	}
	if got, want := args["VITE_NOC_BACKEND_URL"], "https://cb-brazil.example/b/noc-api/api/v1"; got != want {
		t.Errorf("VITE_NOC_BACKEND_URL = %q, want %q", got, want)
	}
	// Keycloak URL is operator-provided and unchanged (never proxied).
	if got, want := args["VITE_KEYCLOAK_URL"], "https://kc.example/auth"; got != want {
		t.Errorf("VITE_KEYCLOAK_URL = %q, want %q (must not be proxied)", got, want)
	}
}

func TestObserveComposeEnv_FrontendOrigin(t *testing.T) {
	t.Setenv("PROXY_TLS_MODE", "internal")
	// Proxy on: backend CORS collapses to the single proxy origin.
	if joined := strings.Join(observeProxyConfig().ComposeEnv(), "\n"); !strings.Contains(joined, "NOC_FRONTEND_ORIGIN=https://cb-brazil.example") {
		t.Errorf("proxy mode must set NOC_FRONTEND_ORIGIN to the proxy origin; got:\n%s", joined)
	}
	// Proxy off: the key is not set (template keeps its "*" local default).
	c := ObserveConfig{FrontendHost: "localhost"}
	c.WithDefaults()
	if joined := strings.Join(c.ComposeEnv(), "\n"); strings.Contains(joined, "NOC_FRONTEND_ORIGIN") {
		t.Errorf("non-proxy mode must not set NOC_FRONTEND_ORIGIN; got:\n%s", joined)
	}
}

func TestObserveSteps_ProxyStepPresence(t *testing.T) {
	has := func(c ObserveConfig) bool {
		c.Runner = &exec.DryRunner{}
		for _, s := range ObserveSteps(c) {
			if s.Name == "start-noc-proxy" {
				return true
			}
		}
		return false
	}
	if !has(observeProxyConfig()) {
		t.Error("proxy-enabled observe must include the start-noc-proxy step")
	}
	off := ObserveConfig{FrontendHost: "localhost"}
	if has(off) {
		t.Error("non-proxy observe must NOT include the start-noc-proxy step")
	}
}

func TestNocProxyStep_WritesFragmentAndAttachesNocNet(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROXY_STATE_DIR", dir)
	fake := &exec.FakeRunner{}
	c := observeProxyConfig()
	c.Runner = fake

	if err := nocProxyStep(c).Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// A SEPARATE fragment file, so the found/join operator fragment is untouched.
	frag, err := os.ReadFile(filepath.Join(dir, "conf.d", "caddy.b-noc.conf"))
	if err != nil {
		t.Fatalf("noc fragment not written: %v", err)
	}
	fs := string(frag)
	if !strings.Contains(fs, "handle_path /b/noc/* {\n\treverse_proxy sc-b-cbweb3-noc-brazil-noc-portal:80\n}") {
		t.Errorf("missing NOC portal route:\n%s", fs)
	}
	if !strings.Contains(fs, "handle_path /b/noc-api/* {\n\treverse_proxy sc-b-cbweb3-noc-brazil-noc-backend:8080\n}") {
		t.Errorf("missing NOC backend route:\n%s", fs)
	}
	var connected bool
	for _, call := range fake.Calls {
		if call.Name == "docker" && strings.HasPrefix(strings.Join(call.Args, " "), "network connect noc-brazil_net "+proxyContainerName) {
			connected = true
		}
	}
	if !connected {
		t.Errorf("expected `docker network connect noc-brazil_net`; calls=%v", fake.Calls)
	}
}

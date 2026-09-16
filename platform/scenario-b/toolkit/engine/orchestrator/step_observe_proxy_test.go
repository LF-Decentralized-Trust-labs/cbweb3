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
	// The portal is no longer told how to reach the realm, and their ABSENCE is now the
	// contract: the login goes to the NOC backend, which performs the grant and sets an
	// HttpOnly cookie. Baking these again would mean the browser can grant for itself,
	// which is what put tokens in localStorage in the first place.
	for _, k := range []string{"VITE_KEYCLOAK_URL", "VITE_KEYCLOAK_REALM", "VITE_KEYCLOAK_CLIENT_ID"} {
		if v, ok := args[k]; ok {
			t.Errorf("%s = %q; the portal must not know the realm — the backend grants", k, v)
		}
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
	// The realm URL is not baked into the portal in either mode any more; it reaches the
	// BACKEND through the compose env, asserted in TestObserveComposeEnv_Keycloak below.
	if v, ok := args["VITE_KEYCLOAK_URL"]; ok {
		t.Errorf("VITE_KEYCLOAK_URL = %q; the portal no longer talks to the realm", v)
	}
}

func TestObserveComposeEnv_FrontendOrigin(t *testing.T) {
	t.Setenv("PROXY_TLS_MODE", "internal")
	// Proxy on: backend CORS collapses to the single proxy origin.
	if joined := strings.Join(observeProxyConfig().ComposeEnv(), "\n"); !strings.Contains(joined, "NOC_FRONTEND_ORIGIN=https://cb-brazil.example") {
		t.Errorf("proxy mode must set NOC_FRONTEND_ORIGIN to the proxy origin; got:\n%s", joined)
	}
	// Proxy off: the key is now ALWAYS set, and names the portal's own origin. It used to
	// be left to the template's "*" default, which a browser refuses to send credentials
	// to — and the session is a cookie now, so a wildcard would leave the portal anonymous
	// after a successful login, with nothing to explain it.
	c := ObserveConfig{FrontendHost: "localhost"}
	c.WithDefaults()
	if joined := strings.Join(c.ComposeEnv(), "\n"); !strings.Contains(joined, "NOC_FRONTEND_ORIGIN=http://localhost:3030") {
		t.Errorf("non-proxy mode must name the portal origin; got:\n%s", joined)
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
	// The upstreams are the network ALIASES the noc-stack template declares, not the container
	// names. They drop the "sc-b-cbweb3-" the container prefix carries, which is the 12 octets
	// that keep the name inside a DNS label as the deployment name grows. The alias and the
	// template are held together by TestComposeAliasesMatchProxyRoutes.
	if !strings.Contains(fs, "handle_path /b/noc/* {\n\treverse_proxy noc-brazil-noc-portal:80\n}") {
		t.Errorf("missing NOC portal route:\n%s", fs)
	}
	if !strings.Contains(fs, "handle_path /b/noc-api/* {\n\treverse_proxy noc-brazil-noc-backend:8080\n}") {
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

// TestObserveComposeEnv_Keycloak pins where the realm configuration went: to the BACKEND,
// which now performs the password grant, and no longer into the portal's build args.
func TestObserveComposeEnv_Keycloak(t *testing.T) {
	c := ObserveConfig{FrontendHost: "localhost", KeycloakURL: "http://localhost:8081"}
	c.WithDefaults()
	joined := strings.Join(c.ComposeEnv(), "\n")

	for _, want := range []string{
		// Translated for the container: the manifest's "localhost" is the browser's view,
		// and the grant now runs inside the NOC backend.
		"NOC_KEYCLOAK_URL=http://host.docker.internal:8081",
		"NOC_KEYCLOAK_REALM=cbweb3",
		"NOC_KEYCLOAK_CLIENT_ID=noc-portal",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("compose env is missing %q; the backend cannot grant without it:\n%s", want, joined)
		}
	}
	// Stable per stack rather than random: a secret that changes on restart refuses every
	// CSRF token issued before it, which looks like a browser fault, not a config one.
	if !strings.Contains(joined, "NOC_CSRF_SECRET=") {
		t.Errorf("compose env carries no NOC_CSRF_SECRET:\n%s", joined)
	}
	if a, b := deriveNOCCSRFSecret("prefix-x"), deriveNOCCSRFSecret("prefix-x"); a != b {
		t.Error("the CSRF secret is not stable for the same stack")
	}
	if a, b := deriveNOCCSRFSecret("prefix-x"), deriveNOCCSRFSecret("prefix-y"); a == b {
		t.Error("two different stacks share a CSRF secret")
	}
}

// TestContainerReachableURL covers the consequence of moving the password grant off the
// browser: spec.noc.keycloakURL was written for a browser, where "localhost" is the host.
// The backend runs in a container, where it is not.
func TestContainerReachableURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"http://localhost:16145", "http://host.docker.internal:16145"},
		{"http://127.0.0.1:16145", "http://host.docker.internal:16145"},
		// Already routable: left exactly as the operator wrote it.
		{"https://kc.example/auth", "https://kc.example/auth"},
		{"http://keycloak.internal:8080", "http://keycloak.internal:8080"},
	}
	for _, tc := range cases {
		if got := containerReachableURL(tc.in); got != tc.want {
			t.Errorf("containerReachableURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestNOCAdminCredential is the fix for a defect that only appeared once NOC_SKIP_AUTH was
// switched off: the toolkit authenticated its admin calls with the literal string
// "local-dev", which passed solely because the no-op validator accepted anything. With real
// validation the deploy failed at register-noc-spoke with 401. The credential has to be a
// real operator from the manifest.
func TestNOCAdminCredential(t *testing.T) {
	c := ObserveConfig{
		FrontendHost: "localhost",
		KeycloakURL:  "http://localhost:16145",
		AdminUsers: []AdminUser{
			{Role: "TREASURY", Username: "treasury@x", Password: "t"},
			{Role: "NOC_ADMIN", Username: "admin@brasil.noc.gov", Password: "brasil-noc-local"},
		},
	}
	c.WithDefaults()

	user, pass, ok := c.nocAdminCredential()
	if !ok {
		t.Fatal("no NOC_ADMIN credential found; the toolkit cannot obtain a token and the " +
			"deploy fails at register-noc-spoke")
	}
	if user != "admin@brasil.noc.gov" || pass != "brasil-noc-local" {
		t.Errorf("credential = %q/%q, want the manifest's NOC_ADMIN", user, pass)
	}
}

// TestNOCAdminCredential_AbsentIsReported keeps the fallback honest: a manifest with no NOC
// operator cannot authenticate, and the caller must be able to say so rather than send a
// string that will be rejected.
func TestNOCAdminCredential_AbsentIsReported(t *testing.T) {
	c := ObserveConfig{FrontendHost: "localhost"}
	c.WithDefaults()
	if _, _, ok := c.nocAdminCredential(); ok {
		t.Error("a config with no NOC_ADMIN reported a credential")
	}
}

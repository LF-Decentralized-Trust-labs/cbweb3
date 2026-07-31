// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-a/toolkit/engine/manifest"
)

func mkManifest(host string) *manifest.Manifest {
	return &manifest.Manifest{Spec: manifest.Spec{FrontendHost: host}}
}

// REGRESSION LOCK: with proxy disabled the NOC portal's backend URL stays the port-based
// local default — the "local & non-proxy deployments unchanged" contract.
func TestNOCBackendURLForManifest_NonProxy_Frozen(t *testing.T) {
	got := nocBackendURLForManifest(mkManifest("localhost"), "localhost", false)
	want := fmt.Sprintf("http://localhost:%d/api/v1", NOCBackendPort)
	if got != want {
		t.Errorf("non-proxy NOC backend URL = %q, want %q", got, want)
	}
}

func TestNOCBackendURLForManifest_Proxy(t *testing.T) {
	t.Setenv("PROXY_TLS_MODE", "internal") // routable host ⇒ https origin
	got := nocBackendURLForManifest(mkManifest("cb-brazil.example"), "cb-brazil.example", true)
	if want := "https://cb-brazil.example/a/noc-api/api/v1"; got != want {
		t.Errorf("proxy NOC backend URL = %q, want %q", got, want)
	}
}

func TestProxyNOCBackendURLScheme(t *testing.T) {
	t.Setenv("PROXY_TLS_MODE", "off") // force http even for a routable host
	if got := proxyNOCBackendURL("cb.example"); got != "http://cb.example/a/noc-api/api/v1" {
		t.Errorf("proxyNOCBackendURL(off) = %q", got)
	}
}

// NewNOCBackendProxyStep writes the SEPARATE caddy.a-noc.conf fragment with the backend
// route (prefix-stripped) — leaving the found operator fragment (caddy.a.conf) untouched.
func TestNewNOCBackendProxyStep_WritesSeparateFragment(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROXY_STATE_DIR", dir)
	// docker calls fail gracefully in the test env; the fragment is written first.
	if err := NewNOCBackendProxyStep("enable", "cb-brazil.example",
		"cbweb3-noc-brazil-net", "cbweb3-noc-brazil-noc-backend:8080").Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	frag, err := os.ReadFile(filepath.Join(dir, "conf.d", "caddy.a-noc.conf"))
	if err != nil {
		t.Fatalf("noc fragment not written: %v", err)
	}
	if !strings.Contains(string(frag), "handle_path /a/noc-api/* {\n\treverse_proxy cbweb3-noc-brazil-noc-backend:8080\n}") {
		t.Errorf("missing NOC backend route:\n%s", frag)
	}
	// The found operator fragment must NOT be created by the observe step.
	if _, err := os.Stat(filepath.Join(dir, "conf.d", "caddy.a.conf")); !os.IsNotExist(err) {
		t.Errorf("observe must not touch the found operator fragment caddy.a.conf")
	}
}

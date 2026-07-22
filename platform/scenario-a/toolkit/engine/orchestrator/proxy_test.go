// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProxyPathHelpersA(t *testing.T) {
	if got := proxyPortalBase("governance"); got != "/a/governance/" {
		t.Fatalf("proxyPortalBase = %q, want /a/governance/", got)
	}
	// A real host ⇒ TLS on ⇒ https-scheme URLs (no mixed content under HTTPS).
	t.Setenv("PROXY_TLS_MODE", "internal")
	if got := proxyAPIURL("cb.example"); got != "https://cb.example/a/api/v1/" {
		t.Fatalf("proxyAPIURL = %q", got)
	}
	if got := proxyAPIBase("cb.example"); got != "https://cb.example/a" {
		t.Fatalf("proxyAPIBase = %q", got)
	}
	if got := proxyOrigin("cb.example"); got != "https://cb.example" {
		t.Fatalf("proxyOrigin = %q", got)
	}
	// PROXY_TLS_MODE=off ⇒ http; localhost ⇒ http.
	t.Setenv("PROXY_TLS_MODE", "off")
	if got := proxyOrigin("cb.example"); got != "http://cb.example" {
		t.Fatalf("proxyOrigin(off) = %q", got)
	}
}

func TestRenderProxyFragmentA(t *testing.T) {
	frag := renderProxyFragment("a", []ProxyRoute{
		{Segment: "governance", Upstream: "cbweb3-central-bank-brazil-governance-frontend:80"},
		{Segment: "api", Upstream: "cbweb3-central-bank-brazil-api-gateway:8080", IsAPI: true},
	}, "")
	if !strings.Contains(frag, "handle_path /a/governance/* {\n\treverse_proxy cbweb3-central-bank-brazil-governance-frontend:80\n}") {
		t.Errorf("missing portal handle_path:\n%s", frag)
	}
	if !strings.Contains(frag, "handle /a/api/* {\n\turi strip_prefix /a\n\treverse_proxy cbweb3-central-bank-brazil-api-gateway:8080 {\n\t\theader_down Set-Cookie \"Path=/\" \"Path=/a\"\n\t}\n}") {
		t.Errorf("missing api handle with cookie-path rewrite:\n%s", frag)
	}
}

func TestCORSOriginsForProxy(t *testing.T) {
	t.Setenv("PROXY_TLS_MODE", "internal") // real host ⇒ TLS on ⇒ https origin
	p := entityPorts(8645)
	// Proxy on → single origin regardless of per-portal ports.
	if got := cbCORSOriginsFor(p, "cb-brazil.example", true); got != "https://cb-brazil.example" {
		t.Errorf("cbCORSOriginsFor(proxy) = %q", got)
	}
	if got := bankCORSOriginsFor(p, "itau.example", true); got != "https://itau.example" {
		t.Errorf("bankCORSOriginsFor(proxy) = %q", got)
	}
	// Proxy on with empty host → defaults to localhost origin (http; localhost never TLS).
	if got := cbCORSOriginsFor(p, "", true); got != "http://localhost" {
		t.Errorf("cbCORSOriginsFor(proxy, empty host) = %q", got)
	}
	// Proxy off → falls back to the per-portal host-port origins.
	if got := cbCORSOriginsFor(p, "localhost", false); got != cbCORSOrigins(p, "localhost") {
		t.Errorf("cbCORSOriginsFor(no proxy) should equal cbCORSOrigins, got %q", got)
	}
}

// TestLauncherProxyModeURLs verifies the launcher fragment carries path-based URLs and
// drops NOC when proxy mode is on. Run writes the fragment before touching docker (which
// fails gracefully in a test env), so no container is created.
func TestLauncherProxyModeURLs(t *testing.T) {
	t.Setenv("PROXY_TLS_MODE", "internal") // real host ⇒ TLS on ⇒ https links
	dir := t.TempDir()
	t.Setenv("LAUNCHER_STATE_DIR", dir)
	step := newLauncherStep("enable", "central-bank", "Central Bank of Brazil",
		"cb-brazil.example", 8645, 5190, true)
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "configs", "config.a.json"))
	if err != nil {
		t.Fatalf("fragment not written: %v", err)
	}
	var frag struct {
		Portals []struct{ Role, URL string }
	}
	if err := json.Unmarshal(raw, &frag); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	byRole := map[string]string{}
	for _, p := range frag.Portals {
		byRole[p.Role] = p.URL
	}
	if got := byRole["governance"]; got != "https://cb-brazil.example/a/governance/" {
		t.Errorf("governance URL = %q", got)
	}
	if _, ok := byRole["noc"]; ok {
		t.Errorf("NOC should be dropped in proxy mode, got %q", byRole["noc"])
	}
}

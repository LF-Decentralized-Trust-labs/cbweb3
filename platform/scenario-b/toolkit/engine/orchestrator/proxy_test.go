package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

func TestProxyPathHelpers(t *testing.T) {
	if got := proxyPortalBase("governance"); got != "/b/governance/" {
		t.Fatalf("proxyPortalBase = %q, want /b/governance/", got)
	}
	if got := proxyAPIURL("cb.example"); got != "http://cb.example/b/api/v1/" {
		t.Fatalf("proxyAPIURL = %q", got)
	}
	if got := proxyOrigin("cb.example"); got != "http://cb.example" {
		t.Fatalf("proxyOrigin = %q", got)
	}
}

func TestRenderProxyFragment(t *testing.T) {
	frag := renderProxyFragment("b", []ProxyRoute{
		{Segment: "governance", Upstream: "pfx-central-bank-governance-frontend:80"},
		{Segment: "api", Upstream: "pfx-central-bank-api-gateway:8080", IsAPI: true},
	}, "")
	// Portal: redirect + prefix-stripping handle_path.
	if !strings.Contains(frag, "redir /b/governance /b/governance/") {
		t.Errorf("missing portal redirect:\n%s", frag)
	}
	if !strings.Contains(frag, "handle_path /b/governance/* {\n\treverse_proxy pfx-central-bank-governance-frontend:80\n}") {
		t.Errorf("missing portal handle_path:\n%s", frag)
	}
	// API: strip only /b, keep /api/v1/…, and scope the auth cookie to /b.
	if !strings.Contains(frag, "handle /b/api/* {\n\turi strip_prefix /b\n\treverse_proxy pfx-central-bank-api-gateway:8080 {\n\t\theader_down Set-Cookie \"Path=/\" \"Path=/b\"\n\t}\n}") {
		t.Errorf("missing api handle with cookie-path rewrite:\n%s", frag)
	}
}

func TestRenderProxyFragmentRootRedirect(t *testing.T) {
	frag := renderProxyFragment("b", nil, "/b/governance/")
	if !strings.Contains(frag, "redir / /b/governance/") {
		t.Errorf("missing root redirect:\n%s", frag)
	}
}

func TestProxyStepEnableWritesFragmentAndWiresDocker(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROXY_STATE_DIR", dir)
	fake := &exec.FakeRunner{}
	step := NewProxyStep(ProxyParams{
		Runner:       fake,
		Mode:         "enable",
		LauncherPort: 5190,
		Networks:     []string{"central-bank-brazil_net"},
		Routes: []ProxyRoute{
			{Segment: "governance", Upstream: "c-governance-frontend:80"},
			{Segment: "api", Upstream: "c-api-gateway:8080", IsAPI: true},
		},
	})
	if err := step.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Fragment written for scenario b.
	frag, err := os.ReadFile(filepath.Join(dir, "conf.d", "caddy.b.conf"))
	if err != nil {
		t.Fatalf("fragment not written: %v", err)
	}
	if !strings.Contains(string(frag), "handle_path /b/governance/*") {
		t.Errorf("fragment missing route:\n%s", frag)
	}
	// FakeRunner reports the container as existing (nil error), so the step skips
	// `docker run`, connects the network, and reloads.
	var connected, reloaded bool
	for _, c := range fake.Calls {
		joined := strings.Join(c.Args, " ")
		if c.Name == "docker" && strings.HasPrefix(joined, "network connect central-bank-brazil_net "+proxyContainerName) {
			connected = true
		}
		if c.Name == "docker" && strings.Contains(joined, "exec "+proxyContainerName+" caddy reload") {
			reloaded = true
		}
	}
	if !connected {
		t.Errorf("expected `docker network connect`; calls=%v", fake.Calls)
	}
	if !reloaded {
		t.Errorf("expected `docker exec … caddy reload`; calls=%v", fake.Calls)
	}
}

func TestProxyStepDisableRemovesFragmentAndTearsDown(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROXY_STATE_DIR", dir)
	fake := &exec.FakeRunner{}
	// Seed a fragment, then disable.
	if err := NewProxyStep(ProxyParams{Runner: fake, Mode: "enable", Networks: []string{"n"},
		Routes: []ProxyRoute{{Segment: "bank", Upstream: "c-frontend:80"}}}).Run(context.Background()); err != nil {
		t.Fatalf("enable: %v", err)
	}
	fake.Calls = nil
	if err := NewProxyStep(ProxyParams{Runner: fake, Mode: "disable"}).Run(context.Background()); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "conf.d", "caddy.b.conf")); !os.IsNotExist(err) {
		t.Errorf("fragment should be removed on disable")
	}
	var removed bool
	for _, c := range fake.Calls {
		if c.Name == "docker" && strings.HasPrefix(strings.Join(c.Args, " "), "rm -f "+proxyContainerName) {
			removed = true
		}
	}
	if !removed {
		t.Errorf("expected `docker rm -f %s` when no fragments remain; calls=%v", proxyContainerName, fake.Calls)
	}
}

func TestSpokeConfigProxyWiring(t *testing.T) {
	c := SpokeConfig{ContainerPrefix: "sc-b-cbweb3-central-bank-brazil", Entity: "central-bank",
		NetPrefix: "central-bank-brazil", RPCPort: 8845, FrontendHost: "cb-brazil.example", ProxyEnabled: true}
	if !c.useProxy() {
		t.Fatal("useProxy should be true")
	}
	if c.frontendVariant() != proxyImageVariant {
		t.Fatalf("frontendVariant = %q", c.frontendVariant())
	}
	if c.corsOrigins() != "http://cb-brazil.example" {
		t.Fatalf("corsOrigins = %q", c.corsOrigins())
	}
	if c.NetName() != "central-bank-brazil_net" {
		t.Fatalf("NetName = %q", c.NetName())
	}
	routes := c.ProxyRoutes()
	if len(routes) != 4 {
		t.Fatalf("want 4 routes, got %d", len(routes))
	}
	if routes[0].Upstream != "sc-b-cbweb3-central-bank-brazil-central-bank-governance-frontend:80" {
		t.Errorf("governance upstream = %q", routes[0].Upstream)
	}
	api := routes[3]
	if !api.IsAPI || api.Upstream != "sc-b-cbweb3-central-bank-brazil-central-bank-api-gateway:8080" {
		t.Errorf("api route = %+v", api)
	}
}

func TestSpokeConfigProxyDisabledFallsBack(t *testing.T) {
	c := SpokeConfig{RPCPort: 8845} // no proxy, no host
	if c.useProxy() {
		t.Fatal("useProxy should be false without ProxyEnabled/FrontendHost")
	}
	if c.frontendVariant() != "" {
		t.Fatalf("frontendVariant should be empty, got %q", c.frontendVariant())
	}
	if got := c.corsOrigins(); got != corsOriginsCB(8845, "") {
		t.Fatalf("corsOrigins should fall back to host-port origins, got %q", got)
	}
}

func TestJoinConfigProxyRoutes(t *testing.T) {
	c := JoinConfig{ContainerPrefix: "sc-b-cbweb3-bank-itau", Entity: "bank-itau",
		NetPrefix: "bank-itau", RPCPort: 10545, FrontendHost: "itau.example", ProxyEnabled: true}
	routes := c.ProxyRoutes()
	if len(routes) != 2 || routes[0].Segment != "bank" ||
		routes[0].Upstream != "sc-b-cbweb3-bank-itau-bank-itau-frontend:80" {
		t.Fatalf("bank routes = %+v", routes)
	}
	if c.corsOrigins() != "http://itau.example" {
		t.Fatalf("corsOrigins = %q", c.corsOrigins())
	}
}

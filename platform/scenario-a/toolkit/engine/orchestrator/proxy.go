// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// The per-host reverse proxy is the single HTTP entrypoint for an entity's VM: one Caddy
// container per host, on port 80, routing by URL path so every portal and API is reached
// without a port. Path-based routing serves each portal under /<scenario>/<role>/ and each
// api-gateway under /<scenario>/api/; the launcher answers the site root.
//
// Like the launcher, each scenario's toolkit writes ONLY its own route fragment
// (caddy.<scenario>.conf) into a shared, neutral conf dir; Caddy imports all fragments and
// reloads. So Scenario A and B never touch each other's fragment and the proxy image is
// generic (built once by proxy/build.sh — this step only runs it and reloads it).
const (
	proxyContainerName    = "cbweb3-proxy"
	proxyImage            = "cbweb3/proxy:local"
	proxyDefaultHTTPPort  = 80
	proxyScenario         = "a"
	proxyFragmentFile     = "caddy.a.conf"
	proxyContainerConfDir = "/etc/caddy/conf.d"
	proxyCaddyfile        = "/etc/caddy/Caddyfile"
	// proxyImageVariant marks a per-entity frontend image tag as a base-path-aware
	// (proxy) build so it is never confused with a non-proxy build of the same entity.
	proxyImageVariant = "-proxy"
)

// proxyPortalBase is the URL base path a portal SPA is served under behind the reverse
// proxy, e.g. "/a/governance/" (baked into the SPA as VITE_BASE_PATH).
func proxyPortalBase(role string) string { return "/" + proxyScenario + "/" + role + "/" }

// proxyAPIURL is the api-gateway URL (with the /api/v1/ suffix) a portal calls behind the
// proxy, e.g. "http://cb.example/a/api/v1/".
func proxyAPIURL(host string) string { return "http://" + host + "/" + proxyScenario + "/api/v1/" }

// proxyAPIBase is the bare api-gateway prefix (no /api/v1/) for SPAs that append their own
// path (e.g. the supervisor's apiFetch), e.g. "http://cb.example/a".
func proxyAPIBase(host string) string { return "http://" + host + "/" + proxyScenario }

// proxyOrigin is the single browser origin all of an entity's portals + its api-gateway
// share behind the proxy, e.g. "http://cb.example".
func proxyOrigin(host string) string { return "http://" + host }

// ProxyRoute is one path route the proxy exposes for this entity: a portal SPA or the
// api-gateway. Segment is the path element under /<scenario>/ (e.g. "governance", "bank",
// "api"); Upstream is the target container:port on the entity network.
type ProxyRoute struct {
	Segment  string
	Upstream string
	IsAPI    bool // api routes strip only /<scenario> (gateway keeps /api/v1/…); portals strip the whole prefix
}

// proxyStep is the SOFT per-host reverse-proxy step. Like the launcher it is soft-by-
// logging: Scenario A's engine has no soft flag, so Run never returns an error for a
// non-fatal proxy problem (missing image, docker failure) — it logs and returns nil so the
// entity's operational deploy is never blocked by this accessory.
type proxyStep struct {
	mode         string // "enable" | "disable" | "" (== disable)
	launcherPort int    // launcher host port → proxy's root fallback upstream
	networks     []string
	routes       []ProxyRoute
	rootRedirect string // when set (hub-less roots), redirect "/" here
}

func newProxyStep(mode string, launcherPort int, networks []string, routes []ProxyRoute, rootRedirect string) Step {
	return &proxyStep{mode: mode, launcherPort: launcherPort, networks: networks, routes: routes, rootRedirect: rootRedirect}
}

func (s *proxyStep) Name() string { return StepStartProxy }

// Check always returns false: the step is idempotent and re-runs each apply so the fragment
// stays current (and disable stays effective).
func (s *proxyStep) Check(context.Context) (bool, error) { return false, nil }

func (s *proxyStep) Run(ctx context.Context) error {
	confDir := filepath.Join(proxyStateDir(), "conf.d")
	fragPath := filepath.Join(confDir, proxyFragmentFile)

	// disable (or absent): drop our fragment; tear the proxy down if none remain.
	if s.mode != "enable" {
		_ = os.Remove(fragPath)
		if proxyNoFragments(confDir) {
			_ = exec.CommandContext(ctx, "docker", "rm", "-f", proxyContainerName).Run()
		}
		return nil
	}

	if err := os.MkdirAll(confDir, 0o755); err != nil {
		return fmt.Errorf("proxy: create conf dir: %w", err)
	}
	if err := os.WriteFile(fragPath, []byte(renderProxyFragment(proxyScenario, s.routes, s.rootRedirect)), 0o644); err != nil {
		return fmt.Errorf("proxy: write fragment: %w", err)
	}

	// Ensure the single per-host proxy container is running (soft: log and continue on
	// failure). It reads fragments at runtime; if already up (possibly started by the
	// other scenario), the reload below picks up the new fragment.
	if !proxyContainerExists(ctx) {
		if !proxyImageExists(ctx) {
			fmt.Fprintf(os.Stderr, "proxy: image %q not found; build it with proxy/build.sh, then re-apply\n", proxyImage)
			return nil
		}
		launcherUpstream := fmt.Sprintf("host.docker.internal:%d", launcherPort(s.launcherPort))
		cmd := exec.CommandContext(ctx, "docker", "run", "-d",
			"--name", proxyContainerName,
			"--restart", "always",
			"-p", fmt.Sprintf("%d:80", proxyHTTPPort()),
			"--add-host", "host.docker.internal:host-gateway",
			"-e", "LAUNCHER_UPSTREAM="+launcherUpstream,
			"-v", confDir+":"+proxyContainerConfDir+":ro",
			proxyImage)
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "proxy: docker run failed (non-fatal): %v\n%s\n", err, out)
			return nil
		}
	}

	// Attach the proxy to this entity's docker network(s) so it can reach the portal / api
	// containers by name. Idempotent: "already exists in network" is not an error.
	for _, net := range s.networks {
		if net == "" {
			continue
		}
		if out, err := exec.CommandContext(ctx, "docker", "network", "connect", net, proxyContainerName).CombinedOutput(); err != nil {
			if !strings.Contains(string(out), "already exists") {
				fmt.Fprintf(os.Stderr, "proxy: network connect %s (non-fatal): %v\n%s\n", net, err, out)
			}
		}
	}

	// Reload so a proxy that was already running (this scenario re-applied, or the other
	// scenario started it) picks up the current fragment set and newly attached networks.
	if out, err := exec.CommandContext(ctx, "docker", "exec", proxyContainerName, "caddy", "reload", "--config", proxyCaddyfile).CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "proxy: caddy reload (non-fatal): %v\n%s\n", err, out)
	}
	return nil
}

// proxyHTTPPort resolves the proxy's host port: PROXY_HTTP_PORT env (override for
// local/testing), else 80. One proxy per host, so no per-entity keying.
func proxyHTTPPort() int {
	if v := os.Getenv("PROXY_HTTP_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			return p
		}
	}
	return proxyDefaultHTTPPort
}

// proxyStateDir is the neutral, per-host conf dir shared by both scenarios' toolkits.
// PROXY_STATE_DIR overrides it explicitly (tests / non-default layouts).
func proxyStateDir() string {
	if d := os.Getenv("PROXY_STATE_DIR"); d != "" {
		return d
	}
	base := "/tmp/cbweb3-proxy"
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		base = filepath.Join(home, ".cbweb3-proxy")
	}
	return base
}

// renderProxyFragment builds the Caddy route block for one scenario: a redirect +
// handle_path per portal (prefix stripped so the SPA's nginx serves at /) and a handle per
// api-gateway (only /<scenario> stripped, so the gateway keeps /api/v1/…).
func renderProxyFragment(scenario string, routes []ProxyRoute, rootRedirect string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Scenario %s routes — generated by the toolkit (proxy: enable). Do not edit.\n", strings.ToUpper(scenario))
	if rootRedirect != "" {
		fmt.Fprintf(&b, "redir / %s\n", rootRedirect)
	}
	for _, r := range routes {
		if r.Upstream == "" || r.Segment == "" {
			continue
		}
		if r.IsAPI {
			// /<scn>/api/* → strip only /<scn>; gateway serves /api/v1/…. Rewrite the
			// auth cookie Path (gateway sets Path=/) to /<scn> so the two scenarios'
			// api-gateways, which share ONE host origin behind the proxy, do not clobber
			// each other's `access_token` / `refresh_token` cookie. The cookie is still
			// sent for this scenario's own portals + api (all under /<scn>/).
			fmt.Fprintf(&b, "handle /%s/%s/* {\n\turi strip_prefix /%s\n\treverse_proxy %s {\n\t\theader_down Set-Cookie \"Path=/\" \"Path=/%s\"\n\t}\n}\n",
				scenario, r.Segment, scenario, r.Upstream, scenario)
			continue
		}
		prefix := fmt.Sprintf("/%s/%s", scenario, r.Segment)
		fmt.Fprintf(&b, "redir %s %s/\n", prefix, prefix)
		fmt.Fprintf(&b, "handle_path %s/* {\n\treverse_proxy %s\n}\n", prefix, r.Upstream)
	}
	return b.String()
}

func proxyContainerExists(ctx context.Context) bool {
	return exec.CommandContext(ctx, "docker", "container", "inspect", proxyContainerName).Run() == nil
}

func proxyImageExists(ctx context.Context) bool {
	return exec.CommandContext(ctx, "docker", "image", "inspect", proxyImage).Run() == nil
}

// proxyNoFragments reports whether confDir holds no caddy.*.conf fragment.
func proxyNoFragments(confDir string) bool {
	entries, err := os.ReadDir(confDir)
	if err != nil {
		return true
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "caddy.") && strings.HasSuffix(e.Name(), ".conf") {
			return false
		}
	}
	return true
}

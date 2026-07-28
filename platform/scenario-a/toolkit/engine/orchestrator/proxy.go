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
	proxyDataVolume       = "cbweb3-proxy-data"
	proxyDefaultHTTPPort  = 80
	proxyScenario         = "a"
	proxyFragmentFile     = "caddy.a.conf"
	proxyContainerConfDir = "/etc/caddy/conf.d"
	proxyContainerCertDir = "/certs"
	proxyCaddyfile        = "/etc/caddy/Caddyfile"
)

// proxyTLSEnabled reports whether the proxy serves this entity over HTTPS. TLS is on when
// the browser-facing host is a real, routable name (not localhost) — the LNET deploy case —
// unless PROXY_TLS_MODE=off forces plain HTTP (e.g. an external edge terminates TLS).
func proxyTLSEnabled(host string) bool {
	switch host {
	case "", "localhost", "127.0.0.1":
		return false
	}
	return os.Getenv("PROXY_TLS_MODE") != "off"
}

// proxyScheme is "https" when the proxy terminates TLS for host, else "http". Baked into
// the SPA's api URL + CORS origin so an HTTPS page never makes a blocked mixed-content call.
func proxyScheme(host string) string {
	if proxyTLSEnabled(host) {
		return "https"
	}
	return "http"
}

// proxyImageVariant tags a per-entity frontend image by its baked-URL flavour (http vs
// https build), so toggling TLS never reuses a stale image cached under the same tag.
func proxyImageVariant(host string) string {
	if proxyTLSEnabled(host) {
		return "-proxytls"
	}
	return "-proxy"
}

// proxyTLSDirective renders the Caddy `tls` directive for PROXY_TLS_MODE: "internal"
// (default; Caddy local CA), "acme" (empty ⇒ automatic Let's Encrypt via HTTP/TLS-ALPN),
// "cloudflare" (empty ⇒ automatic Let's Encrypt via the DNS-01 challenge; the challenge
// provider is set globally by proxyGlobalOptions), or "custom" (operator cert at /certs).
func proxyTLSDirective() string {
	switch os.Getenv("PROXY_TLS_MODE") {
	case "acme", "cloudflare":
		return ""
	case "custom":
		return "tls " + proxyContainerCertDir + "/proxy.crt " + proxyContainerCertDir + "/proxy.key"
	default:
		return "tls internal"
	}
}

// proxyGlobalOptions returns the Caddy global-options line for the current TLS mode. For
// "cloudflare" it enables the ACME DNS-01 challenge via the Cloudflare provider, reading
// the API token from the container's CF_API_TOKEN at runtime — this is what lets a host
// that is not publicly reachable (the LNET/VPN case) obtain a publicly trusted certificate.
// Empty for every other mode (an empty global-options block is valid and a no-op).
func proxyGlobalOptions() string {
	if os.Getenv("PROXY_TLS_MODE") == "cloudflare" {
		return "acme_dns cloudflare {env.CF_API_TOKEN}"
	}
	return ""
}

// proxyCFToken is the Cloudflare API token (scoped to the DNS zone) used by the DNS-01
// challenge in "cloudflare" mode. Set CF_API_TOKEN on the deploy host before apply.
func proxyCFToken() string { return os.Getenv("CF_API_TOKEN") }

// proxyPortalBase is the URL base path a portal SPA is served under behind the reverse
// proxy, e.g. "/a/governance/" (baked into the SPA as VITE_BASE_PATH).
func proxyPortalBase(role string) string { return "/" + proxyScenario + "/" + role + "/" }

// proxyAPIURL is the api-gateway URL (with the /api/v1/ suffix) a portal calls behind the
// proxy, e.g. "https://cb.example/a/api/v1/".
func proxyAPIURL(host string) string {
	return proxyScheme(host) + "://" + host + "/" + proxyScenario + "/api/v1/"
}

// proxyAPIBase is the bare api-gateway prefix (no /api/v1/) for SPAs that append their own
// path (e.g. the supervisor's apiFetch), e.g. "https://cb.example/a".
func proxyAPIBase(host string) string { return proxyScheme(host) + "://" + host + "/" + proxyScenario }

// proxyOrigin is the single browser origin all of an entity's portals + its api-gateway
// share behind the proxy, e.g. "https://cb.example".
func proxyOrigin(host string) string { return proxyScheme(host) + "://" + host }

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
	siteHost     string // browser-facing host (spec.frontendHost); a real host ⇒ serve HTTPS on :443
	launcherPort int    // launcher host port → proxy's root fallback upstream
	networks     []string
	routes       []ProxyRoute
	rootRedirect string // when set (hub-less roots), redirect "/" here
}

func newProxyStep(mode, siteHost string, launcherPort int, networks []string, routes []ProxyRoute, rootRedirect string) Step {
	return &proxyStep{mode: mode, siteHost: siteHost, launcherPort: launcherPort, networks: networks, routes: routes, rootRedirect: rootRedirect}
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
	// other scenario), the reload below picks up the new fragment. When TLS is wanted but
	// the running container predates it (no :443 binding), recreate it so it serves HTTPS;
	// each scenario re-attaches its own network on its next apply.
	tls := proxyTLSEnabled(s.siteHost)
	needCreate := !proxyContainerExists(ctx)
	if !needCreate && tls && !proxyPublishes(ctx, "443") {
		_ = exec.CommandContext(ctx, "docker", "rm", "-f", proxyContainerName).Run()
		needCreate = true
	}
	if needCreate {
		if !proxyImageExists(ctx) {
			fmt.Fprintf(os.Stderr, "proxy: image %q not found; build it with proxy/build.sh, then re-apply\n", proxyImage)
			return nil
		}
		launcherUpstream := fmt.Sprintf("host.docker.internal:%d", launcherPort(s.launcherPort))
		args := []string{"run", "-d",
			"--name", proxyContainerName,
			"--restart", "always",
			"--add-host", "host.docker.internal:host-gateway",
			"-e", "LAUNCHER_UPSTREAM=" + launcherUpstream,
			"-v", confDir + ":" + proxyContainerConfDir + ":ro"}
		if tls {
			// HTTPS: publish :80 (ACME challenge + →:443 redirect) and :443; persist the
			// cert store (ACME / internal CA root) in a named volume across restarts.
			args = append(args,
				"-p", "80:80", "-p", "443:443",
				"-e", "PROXY_SITE="+s.siteHost,
				"-e", "PROXY_TLS_DIRECTIVE="+proxyTLSDirective(),
				"-v", proxyDataVolume+":/data")
			if os.Getenv("PROXY_TLS_MODE") == "cloudflare" {
				// DNS-01 via Cloudflare: enable the challenge globally and hand the scoped
				// API token to the container. Warn (soft) if the token is missing so the
				// operator fixes it rather than silently falling back.
				if proxyCFToken() == "" {
					fmt.Fprintln(os.Stderr, "proxy: PROXY_TLS_MODE=cloudflare but CF_API_TOKEN is empty; set it on the deploy host before apply")
				}
				args = append(args,
					"-e", "PROXY_GLOBAL_OPTIONS="+proxyGlobalOptions(),
					"-e", "CF_API_TOKEN="+proxyCFToken())
			}
			if dir := os.Getenv("PROXY_CERT_DIR"); dir != "" {
				args = append(args, "-v", dir+":"+proxyContainerCertDir+":ro")
			}
		} else {
			args = append(args, "-p", fmt.Sprintf("%d:80", proxyHTTPPort()))
		}
		args = append(args, proxyImage)
		if out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput(); err != nil {
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

// proxyPublishes reports whether the running proxy container publishes the given host
// port (used to detect an HTTP-only proxy that predates a TLS enable and must be recreated).
func proxyPublishes(ctx context.Context, port string) bool {
	out, err := exec.CommandContext(ctx, "docker", "port", proxyContainerName, port+"/tcp").Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
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

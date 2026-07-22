package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// The per-host reverse proxy is the single HTTP entrypoint for an entity's VM: one
// Caddy container per host, on port 80, routing by URL path so every portal and API is
// reached without a port. Path-based routing serves each portal under /<scenario>/<role>/
// and each api-gateway under /<scenario>/api/; the launcher answers the site root.
//
// Like the launcher, each scenario's toolkit writes ONLY its own route fragment
// (caddy.<scenario>.conf) into a shared, neutral conf dir; Caddy imports all fragments
// and reloads. So Scenario A and B never touch each other's fragment and the proxy image
// is generic (built once by proxy/build.sh — this step only runs it and reloads it).
const (
	proxyContainerName    = "cbweb3-proxy"
	proxyImage            = "cbweb3/proxy:local"
	proxyDataVolume       = "cbweb3-proxy-data"
	proxyDefaultHTTPPort  = 80
	proxyScenario         = "b"
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
// (default; Caddy local CA), "acme" (empty ⇒ automatic Let's Encrypt), or "custom"
// (operator cert mounted at /certs).
func proxyTLSDirective() string {
	switch os.Getenv("PROXY_TLS_MODE") {
	case "acme":
		return ""
	case "custom":
		return "tls " + proxyContainerCertDir + "/proxy.crt " + proxyContainerCertDir + "/proxy.key"
	default:
		return "tls internal"
	}
}

// proxyPortalBase is the URL base path a portal SPA is served under behind the reverse
// proxy, e.g. "/b/governance/". Baked into the SPA (VITE_BASE_PATH) so its assets and
// client-side router resolve under the prefix.
func proxyPortalBase(role string) string { return "/" + proxyScenario + "/" + role + "/" }

// proxyAPIURL is the api-gateway URL a portal SPA calls behind the proxy, e.g.
// "https://cb-brazil.example/b/api/v1/" — same origin as the portals (one CORS origin).
func proxyAPIURL(host string) string {
	return proxyScheme(host) + "://" + host + "/" + proxyScenario + "/api/v1/"
}

// proxyOrigin is the single browser origin all of an entity's portals + its api-gateway
// share behind the proxy, e.g. "https://cb-brazil.example".
func proxyOrigin(host string) string { return proxyScheme(host) + "://" + host }

// ProxyRoute is one path route the proxy exposes for this entity: a portal SPA or the
// api-gateway. Segment is the path element under /<scenario>/ (e.g. "governance", "noc",
// "bank", "api"); Upstream is the target container:port on the entity network.
type ProxyRoute struct {
	Segment  string
	Upstream string
	IsAPI    bool // api routes strip only /<scenario> (gateway keeps /api/v1/…); portals strip the whole prefix
}

// ProxyParams configures the per-host reverse-proxy step.
type ProxyParams struct {
	Runner       exec.CommandRunner
	Mode         string   // "enable" | "disable" | "" (== disable)
	SiteHost     string   // browser-facing host (spec.frontendHost); a real host ⇒ serve HTTPS on :443
	LauncherPort int      // launcher host port → proxy's root fallback upstream (host.docker.internal:<port>)
	Networks     []string // entity docker networks to attach the proxy to (reach upstreams by name)
	Routes       []ProxyRoute
	// RootRedirect, when set, redirects the site root "/" to this path. Used by the hub,
	// which has no launcher landing page — root lands on its portal (e.g. "/b/governance/").
	RootRedirect string
}

// NewProxyStep builds the SOFT per-host reverse-proxy step. Like the launcher it never
// blocks the entity's operational deploy: a missing proxy image or docker error is
// non-fatal. Appended by apply after the found-spoke / join step list.
func NewProxyStep(p ProxyParams) Step {
	return Step{
		Name: "start-proxy",
		Soft: true,
		Run:  func(ctx context.Context) error { return runProxyB(ctx, p) },
	}
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

func runProxyB(ctx context.Context, p ProxyParams) error {
	confDir := filepath.Join(proxyStateDir(), "conf.d")
	fragPath := filepath.Join(confDir, "caddy."+proxyScenario+".conf")

	// disable (or absent): drop our fragment; tear the proxy down if none remain.
	if p.Mode != "enable" {
		_ = os.Remove(fragPath)
		if proxyNoFragments(confDir) {
			_, _ = p.Runner.Run(ctx, "docker", "rm", "-f", proxyContainerName)
		}
		return nil
	}

	// enable: write this scenario's route fragment.
	if err := os.MkdirAll(confDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(fragPath, []byte(renderProxyFragment(proxyScenario, p.Routes, p.RootRedirect)), 0o644); err != nil {
		return err
	}

	// Ensure the single per-host proxy container is running. It reads fragments at
	// runtime; if it is already up (possibly started by the other scenario) there is
	// nothing to create — the reload below picks up the new fragment. When TLS is wanted
	// but the running container predates it (no :443 binding), recreate it so it serves
	// HTTPS; each scenario re-attaches its own network on its next apply.
	tls := proxyTLSEnabled(p.SiteHost)
	needCreate := !proxyContainerExists(ctx, p.Runner)
	if !needCreate && tls && !proxyPublishes(ctx, p.Runner, "443") {
		_, _ = p.Runner.Run(ctx, "docker", "rm", "-f", proxyContainerName)
		needCreate = true
	}
	if needCreate {
		if !imageExists(ctx, p.Runner, proxyImage) {
			return fmt.Errorf("proxy image %q not found; build it first with proxy/build.sh", proxyImage)
		}
		launcherUpstream := fmt.Sprintf("host.docker.internal:%d", launcherPort(p.LauncherPort))
		args := []string{"run", "-d",
			"--name", proxyContainerName,
			"--restart", "always",
			"--add-host", "host.docker.internal:host-gateway",
			"-e", "LAUNCHER_UPSTREAM=" + launcherUpstream,
			"-v", confDir + ":" + proxyContainerConfDir + ":ro"}
		if tls {
			// HTTPS: publish :80 (ACME challenge + →:443 redirect) and :443; persist the
			// cert store (ACME/internal CA root) in a named volume so restarts reuse it.
			args = append(args,
				"-p", "80:80", "-p", "443:443",
				"-e", "PROXY_SITE="+p.SiteHost,
				"-e", "PROXY_TLS_DIRECTIVE="+proxyTLSDirective(),
				"-v", proxyDataVolume+":/data")
			if dir := os.Getenv("PROXY_CERT_DIR"); dir != "" {
				args = append(args, "-v", dir+":"+proxyContainerCertDir+":ro")
			}
		} else {
			args = append(args, "-p", fmt.Sprintf("%d:80", proxyHTTPPort()))
		}
		args = append(args, proxyImage)
		if _, err := p.Runner.Run(ctx, "docker", args...); err != nil {
			return err
		}
	}

	// Attach the proxy to this entity's docker network(s) so it can reach the portal /
	// api containers by name. Idempotent: "already exists in network" is not an error.
	for _, net := range p.Networks {
		if net == "" {
			continue
		}
		if _, err := p.Runner.Run(ctx, "docker", "network", "connect", net, proxyContainerName); err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				fmt.Fprintf(os.Stderr, "proxy: network connect %s (non-fatal): %v\n", net, err)
			}
		}
	}

	// Reload so a proxy that was already running (this scenario re-applied, or the other
	// scenario started it) picks up the current fragment set and newly attached networks.
	if _, err := p.Runner.Run(ctx, "docker", "exec", proxyContainerName, "caddy", "reload", "--config", proxyCaddyfile); err != nil {
		fmt.Fprintf(os.Stderr, "proxy: caddy reload (non-fatal): %v\n", err)
	}
	return nil
}

// renderProxyFragment builds the Caddy route block for one scenario: a redirect +
// handle_path per portal (prefix stripped so the SPA's nginx serves at /) and a handle
// per api-gateway (only /<scenario> stripped, so the gateway keeps /api/v1/…).
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
		// /<scn>/<role>/* → strip the whole prefix; SPA nginx serves at /.
		prefix := fmt.Sprintf("/%s/%s", scenario, r.Segment)
		fmt.Fprintf(&b, "redir %s %s/\n", prefix, prefix)
		fmt.Fprintf(&b, "handle_path %s/* {\n\treverse_proxy %s\n}\n", prefix, r.Upstream)
	}
	return b.String()
}

func proxyContainerExists(ctx context.Context, r exec.CommandRunner) bool {
	_, err := r.Run(ctx, "docker", "container", "inspect", proxyContainerName)
	return err == nil
}

// proxyPublishes reports whether the running proxy container publishes the given host
// port (used to detect an HTTP-only proxy that predates a TLS enable and must be recreated).
func proxyPublishes(ctx context.Context, r exec.CommandRunner, port string) bool {
	out, err := r.Run(ctx, "docker", "port", proxyContainerName, port+"/tcp")
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

// proxyNoFragments reports whether confDir holds no caddy.*.conf fragment (so the proxy
// has nothing left to route and can be torn down).
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

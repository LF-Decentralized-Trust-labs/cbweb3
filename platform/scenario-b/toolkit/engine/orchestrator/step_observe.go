package orchestrator

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/bundle"
	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

const (
	// nocLocalAdminBearer is any non-empty bearer token. The toolkit only
	// provisions LOCAL NOC backends running with NOC_SKIP_AUTH=true, whose NoOp
	// Keycloak client accepts any token and grants ROLE_NOC_ADMIN. Production NOC
	// backends are provisioned out of band, never by the toolkit.
	nocLocalAdminBearer = "local-dev"
	// nocFoundingAgentLabel is the fixed entity label for the founding CB's (or
	// hub's) agent key. observe (which provisions it) and found-* (which uses it)
	// both derive the SAME key from the spoke id + this label, with no exchange.
	nocFoundingAgentLabel = "cb"

	// defaultNOCBackendPort / defaultNOCPortalPort are the fixed host ports the
	// observe NOC control plane publishes (one NOC per host). They are fixed (not
	// derived from a spoke's Besu RPC port) because the NOC is a standalone
	// participant; the per-entity launcher links the NOC portal at this port.
	defaultNOCBackendPort = 8090
	defaultNOCPortalPort  = 3030
)

// ObserveConfig drives an observe-mode NOC deployment: it stands up the NOC
// control plane (db + backend + portal), then registers the spoke described by
// the consumed NOC bundle and provisions the founding agent's key. Agents run
// with each entity and push here (multi-agent per spoke).
type ObserveConfig struct {
	Runner          exec.CommandRunner
	ScenarioBDir    string // docker build context root for the NOC images
	TemplatesDir    string
	Bundle          bundle.NOCBundle
	ContainerPrefix string
	NetPrefix       string
	VolumePrefix    string
	BackendPort     int
	PortalPort      int
	FrontendHost    string
	// KeycloakURL is the routable CB/hub Keycloak the portal password-grants
	// against (spec.noc.keycloakURL). Baked as the portal's VITE_KEYCLOAK_URL.
	KeycloakURL string
	// LauncherURL is baked as the portal's VITE_LAUNCHER_URL (back-to-launcher).
	LauncherURL string
	// ProxyEnabled (spec.proxy == "enable") serves the NOC portal + its backend API
	// behind the per-host reverse proxy under /b/noc/ and /b/noc-api/ on the single
	// proxy origin. When false the portal + backend stay port-based (local default).
	// The portal's Keycloak URL is unchanged either way — it is the operator-provided
	// routable CB/hub realm (spec.noc.keycloakURL), reached on its own origin, not proxied.
	ProxyEnabled bool

	// BackendURL is where the toolkit reaches the backend to register/provision
	// (default http://localhost:<BackendPort>). Injectable for tests.
	BackendURL string
	// HTTPClient defaults to http.DefaultClient. Injectable for tests.
	HTTPClient *http.Client
	// WaitBackend gates registration on backend readiness (default: poll /health).
	WaitBackend func(ctx context.Context) error
}

// WithDefaults fills unset fields with local single-host conventions.
func (c *ObserveConfig) WithDefaults() {
	if c.ContainerPrefix == "" {
		c.ContainerPrefix = "sc-b-cbweb3-noc"
	}
	if c.NetPrefix == "" {
		c.NetPrefix = c.ContainerPrefix
	}
	if c.VolumePrefix == "" {
		c.VolumePrefix = c.ContainerPrefix
	}
	if c.BackendPort == 0 {
		c.BackendPort = defaultNOCBackendPort
	}
	if c.PortalPort == 0 {
		c.PortalPort = defaultNOCPortalPort
	}
	if c.FrontendHost == "" {
		c.FrontendHost = "localhost"
	}
	if c.BackendURL == "" {
		c.BackendURL = fmt.Sprintf("http://localhost:%d", c.BackendPort)
	}
	if c.HTTPClient == nil {
		c.HTTPClient = http.DefaultClient
	}
	if c.WaitBackend == nil {
		url := strings.TrimRight(c.BackendURL, "/") + "/health"
		c.WaitBackend = func(ctx context.Context) error { return waitHTTPOK(ctx, url, 60*time.Second) }
	}
}

// ComposeEnv is the process env the noc-stack template resolves its ${...} from.
func (c ObserveConfig) ComposeEnv() []string {
	vars := map[string]string{
		"CONTAINER_PREFIX":  c.ContainerPrefix,
		"NOC_DB_USER":       "cbweb3",
		"NOC_DB_PASSWORD":   "cbweb3",
		"NOC_DB_NAME":       "noc",
		"NOC_BACKEND_PORT":  itoa(c.BackendPort),
		"NOC_PORTAL_PORT":   itoa(c.PortalPort),
		"NOC_NET_PREFIX":    c.NetPrefix,
		"NOC_VOLUME_PREFIX": c.VolumePrefix,
		"NOC_BACKEND_IMAGE": hubNocBackendImage,
		"NOC_PORTAL_IMAGE":  c.portalImage(),
	}
	if c.ProxyEnabled {
		// Behind the proxy the portal is same-origin with the backend, so the backend's
		// browser CORS collapses to the single proxy origin (vs the local "*" default).
		vars["NOC_FRONTEND_ORIGIN"] = proxyOrigin(c.FrontendHost)
	}
	out := make([]string, 0, len(vars))
	for k, v := range vars {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}

// portalViteArgs are the build args baked into the NOC portal SPA (read at build
// time). The backend URL is the BROWSER-reachable one: the FrontendHost + published
// port (port-based default), or the same-origin proxy path when ProxyEnabled — the
// latter is what keeps an HTTPS page from making a blocked mixed-content call. It is
// distinct from BackendURL (the toolkit's localhost admin path). VITE_KEYCLOAK_URL is
// the operator-provided routable realm in both modes (never proxied).
func (c ObserveConfig) portalViteArgs() map[string]string {
	args := map[string]string{
		"VITE_NOC_BACKEND_URL":    fmt.Sprintf("http://%s:%d/api/v1", c.FrontendHost, c.BackendPort),
		"VITE_KEYCLOAK_URL":       c.KeycloakURL,
		"VITE_KEYCLOAK_REALM":     spokeKeycloakRealm,
		"VITE_KEYCLOAK_CLIENT_ID": nocKeycloakClient,
		"VITE_LAUNCHER_URL":       c.LauncherURL,
	}
	if c.ProxyEnabled {
		// Served under /b/noc/; assets + router resolve under the prefix.
		args["VITE_BASE_PATH"] = proxyPortalBase("noc")
		// Backend same-origin at /b/noc-api/…; the /b/noc-api prefix is stripped by
		// Caddy (handle_path) so the backend still receives /api/v1/….
		args["VITE_NOC_BACKEND_URL"] = proxyOrigin(c.FrontendHost) + nocProxyAPIBase + "/api/v1"
	}
	return args
}

// portalImage tags the NOC portal image by a hash of its baked VITE args, so
// changing any URL forces a rebuild (buildFrontendImage skips when the exact tag
// already exists — a fixed tag would keep a stale, wrongly-baked image).
func (c ObserveConfig) portalImage() string {
	return "cbweb3b/noc-portal:" + shortHashArgs(c.portalViteArgs())
}

// shortHashArgs is a stable 10-hex digest of a string map (sorted key=value).
func shortHashArgs(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k + "=" + m[k] + ";"))
	}
	return hex.EncodeToString(h.Sum(nil))[:10]
}

func (c ObserveConfig) template(name string) string {
	return filepath.Join(c.TemplatesDir, name+".compose.yaml")
}

func (c ObserveConfig) composeUpArgs() []string {
	return []string{"compose", "-p", c.ContainerPrefix, "-f", c.template("noc-stack"), "up", "-d"}
}

// nocNetName is the dedicated observe network the NOC stack owns (noc-stack.compose.yaml:
// noc_net → <NOC_NET_PREFIX>_net). The proxy attaches to it to reach the portal + backend.
func (c ObserveConfig) nocNetName() string { return c.NetPrefix + "_net" }

// nocProxyStep (soft) wires the NOC portal + backend into the per-host reverse proxy: it
// attaches the proxy to the NOC network and writes the caddy.b-noc.conf fragment with a
// portal route (/b/noc/ → portal:80) and a backend route (/b/noc-api/ → backend:8080, the
// prefix stripped so the backend still serves /api/v1/…). Runs after the stack is up so the
// containers + network exist. Reuses the generic proxy runner, keyed to its own fragment.
func nocProxyStep(c ObserveConfig) Step {
	step := NewProxyStep(ProxyParams{
		Runner:   c.Runner,
		Mode:     "enable",
		SiteHost: c.FrontendHost,
		Fragment: nocProxyFragment,
		Networks: []string{c.nocNetName()},
		Routes: []ProxyRoute{
			{Segment: nocProxyPortalSegment, Upstream: c.ContainerPrefix + "-noc-portal:80"},
			{Segment: nocProxyAPISegment, Upstream: c.ContainerPrefix + "-noc-backend:8080"},
		},
	})
	step.Name = "start-noc-proxy"
	step.Deps = []string{"start-noc-stack"}
	return step
}

// ObserveSteps assembles the observe-mode step DAG.
func ObserveSteps(c ObserveConfig) []Step {
	c.WithDefaults()
	steps := []Step{
		{
			// Backend from its own service dir (buildImageIn, no args). Portal via
			// buildFrontendImage so the VITE_* (backend/keycloak/launcher) are baked;
			// the agent image is NOT built here (agents run with each entity).
			Name: "build-noc-images",
			Check: func(ctx context.Context) (bool, error) {
				return imageExists(ctx, c.Runner, hubNocBackendImage) &&
					imageExists(ctx, c.Runner, c.portalImage()), nil
			},
			Run: func(ctx context.Context) error {
				if err := buildImageIn(ctx, c.Runner, c.ScenarioBDir, hubNocBackendImage,
					"backend/services/noc-backend/Dockerfile", "backend/services/noc-backend"); err != nil {
					return err
				}
				return buildFrontendImage(ctx, c.Runner, c.ScenarioBDir, c.portalImage(), "noc", c.portalViteArgs())
			},
		},
		{
			Name: "start-noc-stack",
			Deps: []string{"build-noc-images"},
			Run: func(ctx context.Context) error {
				_, err := c.Runner.Run(ctx, "docker", c.composeUpArgs()...)
				return err
			},
		},
		{
			Name: "wait-noc-backend",
			Deps: []string{"start-noc-stack"},
			Run:  func(ctx context.Context) error { return c.WaitBackend(ctx) },
		},
		{
			// register-noc-spoke records the spoke in the NOC backend under its
			// deterministic UUID so the portal lists it and agents can push to it.
			Name: "register-noc-spoke",
			Deps: []string{"wait-noc-backend"},
			Check: func(ctx context.Context) (bool, error) {
				return c.spokeRegistered(ctx), nil
			},
			Run: func(ctx context.Context) error {
				return c.registerSpoke(ctx)
			},
		},
		{
			// provision-noc-key binds the founding agent's deterministic key to the
			// spoke. Idempotent server-side (FirstOrCreate on the key hash).
			Name: "provision-noc-key",
			Deps: []string{"register-noc-spoke"},
			Run: func(ctx context.Context) error {
				return c.provisionKey(ctx)
			},
		},
	}
	// Behind the proxy (spec.proxy == enable): route the portal + backend on the single
	// proxy origin. Additive + soft — the port-based deploy is unchanged when disabled.
	if c.ProxyEnabled {
		steps = append(steps, nocProxyStep(c))
	}
	return steps
}

// spokeRegistered reports whether the spoke already exists (idempotency gate).
// A transport error (backend not up) reports "not registered" rather than an
// error, so the step runs and surfaces the real failure.
func (c ObserveConfig) spokeRegistered(ctx context.Context) bool {
	url := strings.TrimRight(c.BackendURL, "/") + "/api/v1/admin/spokes/" + c.Bundle.SpokeUUID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+nocLocalAdminBearer)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (c ObserveConfig) registerSpoke(ctx context.Context) error {
	payload, err := json.Marshal(map[string]any{
		"id":            c.Bundle.SpokeUUID,
		"name":          c.Bundle.Name,
		"currency_code": c.Bundle.CurrencyCode,
		"jurisdiction":  c.Bundle.Jurisdiction,
	})
	if err != nil {
		return err
	}
	// 201 created, 200 restored (soft-deleted), 409 already active — all idempotent OK.
	return c.adminPost(ctx, "/api/v1/admin/spokes", payload,
		http.StatusCreated, http.StatusOK, http.StatusConflict)
}

func (c ObserveConfig) provisionKey(ctx context.Context) error {
	payload, err := json.Marshal(map[string]string{
		"raw_key":  deterministicAgentKey(c.Bundle.SpokeID, nocFoundingAgentLabel),
		"spoke_id": c.Bundle.SpokeUUID,
		"hint":     c.Bundle.SpokeID + "-" + nocFoundingAgentLabel,
	})
	if err != nil {
		return err
	}
	return c.adminPost(ctx, "/api/v1/admin/agents/provision-key", payload,
		http.StatusCreated, http.StatusOK)
}

// adminPost POSTs a JSON body to a NOC admin endpoint, treating any of okCodes
// as success. Mirrors the register-* HTTP step pattern (inline net/http).
func (c ObserveConfig) adminPost(ctx context.Context, path string, body []byte, okCodes ...int) error {
	url := strings.TrimRight(c.BackendURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+nocLocalAdminBearer)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("observe: POST %s: %w", path, err)
	}
	defer resp.Body.Close()
	for _, ok := range okCodes {
		if resp.StatusCode == ok {
			return nil
		}
	}
	b, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("observe: POST %s returned %d: %s", path, resp.StatusCode, strings.TrimSpace(string(b)))
}

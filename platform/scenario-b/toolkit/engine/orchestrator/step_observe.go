// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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
	// AMMGatewayURL is the api-gateway the NOC BACKEND (not the browser) reads AMM
	// pool status from, for the Pool Stability page (spec.noc.ammGatewayURL). The NOC
	// stack owns its own docker network, so another stack's compose service name does
	// not resolve here — this must be reachable from the NOC container, typically the
	// host plus the gateway's published port. Empty leaves the backend default, and
	// the page reports why it is empty instead of pretending the AMM has no pools.
	AMMGatewayURL string
	// LauncherURL is baked as the portal's VITE_LAUNCHER_URL (back-to-launcher).
	LauncherURL string
	// AdminUsers carries the manifest's operators. The NOC_ADMIN among them is what the
	// toolkit authenticates its own admin calls with, now that the NOC backend validates
	// tokens for real.
	AdminUsers []AdminUser
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
// ProxyRoutes is the observe stack's route set, alongside SpokeConfig's, JoinConfig's and
// HubConfig's. A method rather than a literal at the call site so a guard can evaluate it for a
// prefix nobody has deployed — the bound has to hold for the next NOC, not the two that exist.
//
// The upstreams are network ALIASES, not container names. A DNS label stops at 63 octets
// (RFC 1035); a container name repeats the prefix and grows with it, which is what took the
// Costa Rica portals down. PR #226 moved the CB, bank and hub route sets off container names and
// left this one behind — the observe stack has its own template and its own network, so nothing
// about that fix reached it.
func (c ObserveConfig) ProxyRoutes() []ProxyRoute {
	return []ProxyRoute{
		{Segment: nocProxyPortalSegment, Upstream: c.nocPortalAlias() + ":80"},
		{Segment: nocProxyAPISegment, Upstream: c.nocBackendAlias() + ":8080"},
	}
}

// The aliases noc-stack.compose.yaml declares. TestComposeAliasesMatchProxyRoutes holds the two
// halves together: change one without the other and the route renders, the apply succeeds, and
// the portal answers 502.
func (c ObserveConfig) nocPortalAlias() string  { return c.NetPrefix + "-noc-portal" }
func (c ObserveConfig) nocBackendAlias() string { return c.NetPrefix + "-noc-backend" }

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
	// The realm the NOC backend password-grants against. It moved here from the PORTAL's
	// build args when the login moved to the server: the browser no longer performs the
	// grant, because a cookie it cannot read can only be set by a server. Same operator-
	// provided routable URL (spec.noc.keycloakURL), now consumed by the backend.
	if c.KeycloakURL != "" {
		vars["NOC_KEYCLOAK_URL"] = containerReachableURL(c.KeycloakURL)
		vars["NOC_KEYCLOAK_REALM"] = spokeKeycloakRealm
		vars["NOC_KEYCLOAK_CLIENT_ID"] = nocKeycloakClient
	}
	// Keys the HMAC binding each CSRF token to its session. Derived from the container
	// prefix so it is stable across restarts of THIS stack — a per-process secret would
	// refuse every token issued before the last restart.
	vars["NOC_CSRF_SECRET"] = deriveNOCCSRFSecret(c.ContainerPrefix)

	// CORS origin. It must name the portal exactly, never "*": a browser refuses to send
	// credentials to a wildcard origin, and the session is a cookie now — so a wildcard
	// would let the portal log in and then be anonymous on every request, with no error
	// anywhere to explain it.
	if c.ProxyEnabled {
		// Behind the proxy the portal is same-origin with the backend.
		vars["NOC_FRONTEND_ORIGIN"] = proxyOrigin(c.FrontendHost)
	} else {
		vars["NOC_FRONTEND_ORIGIN"] = fmt.Sprintf("http://%s:%d", c.FrontendHost, c.PortalPort)
	}
	if c.AMMGatewayURL != "" {
		// Pool Stability data source. Pairs are discovered from this gateway, so no
		// pair list has to be configured here.
		vars["AMM_GATEWAY_URL"] = c.AMMGatewayURL
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
	// No VITE_KEYCLOAK_* any more: the portal does not talk to the realm. The login goes
	// to its own backend, which performs the grant and sets an HttpOnly cookie — the whole
	// reason the tokens left localStorage.
	args := map[string]string{
		"VITE_NOC_BACKEND_URL": fmt.Sprintf("http://%s:%d/api/v1", c.FrontendHost, c.BackendPort),
		"VITE_LAUNCHER_URL":    c.LauncherURL,
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

// nocAdminBearer returns the token the toolkit authenticates its NOC admin calls with.
//
// It performs the same password grant the NOC backend performs for a browser, using the
// manifest's NOC_ADMIN operator against the same realm. That is what makes the call work
// against a backend that actually validates tokens.
//
// Falls back to the legacy placeholder when the manifest names no NOC operator, or when the
// grant fails: a stack still running NOC_SKIP_AUTH=true accepts anything, so the fallback
// keeps that deployment working instead of failing it on a credential it does not need.
// The failure is logged rather than swallowed, because on a validating backend the next
// call answers 401 and the reason must be visible.
func (c ObserveConfig) nocAdminBearer(ctx context.Context) string {
	user, pass, ok := c.nocAdminCredential()
	if !ok || c.KeycloakURL == "" {
		return nocLocalAdminBearer
	}
	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", nocKeycloakClient)
	form.Set("username", user)
	form.Set("password", pass)

	endpoint := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token",
		strings.TrimRight(c.KeycloakURL, "/"), spokeKeycloakRealm)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		log.Printf("[observe] noc admin token: %v — falling back to the skip-auth placeholder", err)
		return nocLocalAdminBearer
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		log.Printf("[observe] noc admin token: realm unreachable at %s: %v — falling back to the skip-auth placeholder", endpoint, err)
		return nocLocalAdminBearer
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("[observe] noc admin token: realm returned %d for %q — falling back to the skip-auth placeholder", resp.StatusCode, user)
		return nocLocalAdminBearer
	}
	var tr struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil || tr.AccessToken == "" {
		log.Printf("[observe] noc admin token: unreadable grant response — falling back to the skip-auth placeholder")
		return nocLocalAdminBearer
	}
	return tr.AccessToken
}

// nocAdminCredential returns the manifest's NOC_ADMIN operator.
//
// It exists because the toolkit used to authenticate its admin calls with the literal
// string "local-dev" — see the comment on nocLocalAdminBearer. That was never a credential:
// it passed only because NOC_SKIP_AUTH made the validator accept any string. With real
// validation the deploy fails at register-noc-spoke with 401, which is exactly what
// happened the first time the flag was switched off.
//
// Reports whether one was found, rather than returning a placeholder: a caller that cannot
// authenticate should say so, not send something that will be refused.
func (c ObserveConfig) nocAdminCredential() (username, password string, ok bool) {
	for _, u := range c.AdminUsers {
		if strings.EqualFold(u.Role, "NOC_ADMIN") && u.Username != "" {
			return u.Username, u.Password, true
		}
	}
	return "", "", false
}

// containerReachableURL rewrites a host-facing "localhost" URL into one a CONTAINER can
// reach.
//
// spec.noc.keycloakURL was written for the browser, which is where the password grant used
// to run — and "localhost" is exactly right there. The grant now runs in the NOC backend,
// inside a container on its own network, where "localhost" is the container itself. Left
// alone, every existing manifest would break at login with a connection refused and nothing
// pointing at the cause.
//
// The container side is host.docker.internal, which noc-stack.compose.yaml already maps via
// extra_hosts for the same reason AMM_GATEWAY_URL needs it. A URL that already names a
// routable host is untouched.
func containerReachableURL(u string) string {
	for _, local := range []string{"//localhost:", "//127.0.0.1:"} {
		if strings.Contains(u, local) {
			return strings.Replace(u, local, "//host.docker.internal:", 1)
		}
	}
	return u
}

// deriveNOCCSRFSecret produces a stable per-stack CSRF secret.
//
// Derived rather than random so it survives a restart: the secret keys the HMAC that binds
// each CSRF token to its session, so a value that changes on boot refuses every token
// issued before it — a browser holding a perfectly good session suddenly gets 403 on every
// action, with nothing to suggest the cause.
//
// It is not a production secret and does not pretend to be: this is the LOCAL toolkit path,
// where the whole stack's credentials are derived the same way. A production deployment
// supplies CSRF_SECRET itself, which is why the backend reads the env var rather than
// deriving anything of its own.
func deriveNOCCSRFSecret(containerPrefix string) string {
	sum := sha256.Sum256([]byte("cbweb3-noc-csrf:" + containerPrefix))
	return hex.EncodeToString(sum[:])
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
		Routes:   c.ProxyRoutes(),
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
			// The Check looks at the actual containers, never at the persisted state:
			// state lives on the host while the containers live in docker, so anything
			// that removes them (docker rm, a --clean redeploy, a pruned host) leaves
			// state claiming "done" for a stack that is gone. This step would then be
			// skipped, nothing would come up, and the registration steps after it would
			// fail against a backend that was never started.
			Name: "start-noc-stack",
			Deps: []string{"build-noc-images"},
			Check: func(ctx context.Context) (bool, error) {
				return c.stackRunning(ctx), nil
			},
			Run: func(ctx context.Context) error {
				_, err := c.Runner.Run(ctx, "docker", c.composeUpArgs()...)
				return err
			},
		},
		{
			// Readiness is likewise a live property: a backend that answers /health now
			// needs no wait, and one that does not must be waited for regardless of what
			// a previous run recorded.
			Name: "wait-noc-backend",
			Deps: []string{"start-noc-stack"},
			Check: func(ctx context.Context) (bool, error) {
				return c.backendHealthy(ctx), nil
			},
			Run: func(ctx context.Context) error { return c.WaitBackend(ctx) },
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
			// spoke. Idempotent server-side (FirstOrCreate on the key hash), so it runs
			// on every apply: the Check below deliberately never skips.
			//
			// It must not fall back to the persisted state either. State says "done"
			// for the host, while the key lives in the NOC database — wipe or replace
			// that database (a fresh deploy keeping the state file) and the key is gone
			// while the state still claims otherwise. Every agent then gets 401 on
			// push and the portal shows no components at all, with nothing in the
			// report pointing at the cause.
			Name:  "provision-noc-key",
			Deps:  []string{"register-noc-spoke"},
			Check: func(ctx context.Context) (bool, error) { return false, nil },
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

// stackContainerNames are the NOC control-plane containers the compose file creates.
func (c ObserveConfig) stackContainerNames() []string {
	return []string{
		c.ContainerPrefix + "-noc-db",
		c.ContainerPrefix + "-noc-backend",
		c.ContainerPrefix + "-noc-portal",
	}
}

// stackRunning reports whether every NOC container exists and is running. Any missing
// or stopped container means `compose up -d` must run again (it is idempotent for the
// ones already up).
func (c ObserveConfig) stackRunning(ctx context.Context) bool {
	for _, name := range c.stackContainerNames() {
		out, err := c.Runner.Run(ctx, "docker", "container", "inspect", "-f", "{{.State.Running}}", name)
		if err != nil || strings.TrimSpace(string(out)) != "true" {
			return false
		}
	}
	return true
}

// backendHealthy reports whether the backend answers /health right now (a single quick
// probe, not the WaitBackend retry loop).
func (c ObserveConfig) backendHealthy(ctx context.Context) bool {
	url := strings.TrimRight(c.BackendURL, "/") + "/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
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
	req.Header.Set("Authorization", "Bearer "+c.nocAdminBearer(ctx))
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
	req.Header.Set("Authorization", "Bearer "+c.nocAdminBearer(ctx))
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

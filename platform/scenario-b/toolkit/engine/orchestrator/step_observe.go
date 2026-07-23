package orchestrator

import (
	"bytes"
	"context"
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
)

// observeImages are the NOC control-plane images an observe deployment builds.
// The agent image is NOT built here: agents run with each entity (found-*/join)
// and push to this backend.
var observeImages = []struct{ image, dockerfile, context string }{
	{hubNocBackendImage, "backend/services/noc-backend/Dockerfile", "backend/services/noc-backend"},
	{hubNocPortalImage, "frontend/apps/noc/Dockerfile", "frontend"},
}

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
		c.BackendPort = 8090
	}
	if c.PortalPort == 0 {
		c.PortalPort = 3030
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
		"NOC_PORTAL_IMAGE":  hubNocPortalImage,
	}
	out := make([]string, 0, len(vars))
	for k, v := range vars {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}

func (c ObserveConfig) template(name string) string {
	return filepath.Join(c.TemplatesDir, name+".compose.yaml")
}

func (c ObserveConfig) composeUpArgs() []string {
	return []string{"compose", "-p", c.ContainerPrefix, "-f", c.template("noc-stack"), "up", "-d"}
}

// ObserveSteps assembles the observe-mode step DAG.
func ObserveSteps(c ObserveConfig) []Step {
	c.WithDefaults()
	return []Step{
		{
			Name: "build-noc-images",
			Check: func(ctx context.Context) (bool, error) {
				for _, b := range observeImages {
					if !imageExists(ctx, c.Runner, b.image) {
						return false, nil
					}
				}
				return true, nil
			},
			Run: func(ctx context.Context) error {
				for _, b := range observeImages {
					if err := buildImageIn(ctx, c.Runner, c.ScenarioBDir, b.image, b.dockerfile, b.context); err != nil {
						return err
					}
				}
				return nil
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

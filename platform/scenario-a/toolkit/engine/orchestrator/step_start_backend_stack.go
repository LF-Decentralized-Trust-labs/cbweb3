// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// startBackendStackStep brings up an entity's 4 backend services (feature 034)
// from the per-entity backend compose template, then waits for the api-gateway
// health endpoint. Reused by the CB (found) and commercial banks (join). It reads
// the rendered per-entity .env and reaches the entity's dedicated infra by
// container name on the entity network.
type startBackendStackStep struct {
	name            string
	entityPrefix    string
	netName         string
	backendContext  string
	pkiDir          string
	envFile         string
	composePath     string
	bankCode        string
	paladinURL      string
	paladinIdentity string
	cactiURL        string
	imageTag        string
	apiPort         int
	authPort        int
	compliancePort  int
	paymentPort     int
	healthTimeout   time.Duration
	healthInterval  time.Duration
}

func newStartBackendStackStep(name string, p backendStackParams) Step {
	return &startBackendStackStep{
		name:            name,
		entityPrefix:    p.EntityPrefix,
		netName:         p.NetName,
		backendContext:  p.BackendContext,
		pkiDir:          p.PKIDir,
		envFile:         p.EnvFile,
		composePath:     p.ComposePath,
		bankCode:        p.BankCode,
		paladinURL:      p.PaladinURL,
		paladinIdentity: p.PaladinIdentity,
		cactiURL:        p.CactiURL,
		imageTag:        p.ImageTag,
		apiPort:         p.APIPort,
		authPort:        p.AuthPort,
		compliancePort:  p.CompliancePort,
		paymentPort:     p.PaymentPort,
		healthTimeout:   p.HealthTimeout,
		healthInterval:  p.HealthInterval,
	}
}

// backendStackParams groups the (many) inputs to keep the constructor readable.
type backendStackParams struct {
	EntityPrefix    string
	NetName         string
	BackendContext  string
	PKIDir          string
	EnvFile         string
	ComposePath     string
	BankCode        string
	PaladinURL      string
	PaladinIdentity string
	CactiURL        string
	ImageTag        string
	APIPort         int
	AuthPort        int
	CompliancePort  int
	PaymentPort     int
	HealthTimeout   time.Duration
	HealthInterval  time.Duration
}

func (s *startBackendStackStep) Name() string { return s.name }

// Check returns true if the api-gateway liveness endpoint already responds.
// /healthz is the unauthenticated liveness probe; /api/v1/health sits behind the
// auth middleware (returns 401), so it cannot be used as a readiness gate.
func (s *startBackendStackStep) Check(ctx context.Context) (bool, error) {
	return httpHealthy(ctx, s.healthURL()), nil
}

func (s *startBackendStackStep) healthURL() string {
	return fmt.Sprintf("http://localhost:%d/healthz", s.apiPort)
}

func (s *startBackendStackStep) Run(ctx context.Context) error {
	// Pre-create the PKI dir as the host user BEFORE compose mounts it. Docker
	// creates a missing bind-mount source as root, which would then deny the
	// host-user gen-csr step (deferred tail) write access to <dataDir>/pki and
	// leave the api-gateway proxy with no CSR to read at onboarding time.
	if s.pkiDir != "" {
		if err := os.MkdirAll(s.pkiDir, 0o755); err != nil {
			return fmt.Errorf("create PKI dir %s: %w", s.pkiDir, err)
		}
	}

	// Unique compose project per entity (shared template would otherwise reconcile
	// and remove another entity's containers — see step_start_infra.go).
	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", s.entityPrefix+"-backend", "-f", s.composePath, "up", "-d")
	cmd.Env = s.composeEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("compose up backend: %w\noutput:\n%s", err, out)
	}
	url := s.healthURL()
	deadline := time.Now().Add(s.healthTimeout)
	for time.Now().Before(deadline) {
		if httpHealthy(ctx, url) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(s.healthInterval):
		}
	}
	return fmt.Errorf("api-gateway health check timed out after %s", s.healthTimeout)
}

func (s *startBackendStackStep) composeEnv() []string {
	cacti := s.cactiURL
	if cacti == "" {
		cacti = "http://host.docker.internal:4000"
	}
	tag := s.imageTag
	if tag == "" {
		tag = "local"
	}
	env := append(os.Environ(),
		"ENTITY_PREFIX="+s.entityPrefix,
		"ENTITY_NET_NAME="+s.netName,
		"BACKEND_CONTEXT="+s.backendContext,
		"ENTITY_ENV_FILE="+s.envFile,
		"BANK_CODE="+s.bankCode,
		"PALADIN_URL="+s.paladinURL,
		"PALADIN_IDENTITY="+s.paladinIdentity,
		"CACTI_API_URL="+cacti,
		"BACKEND_IMAGE_TAG="+tag,
		"API_GATEWAY_PORT="+strconv.Itoa(s.apiPort),
		"AUTH_PORT="+strconv.Itoa(s.authPort),
		"COMPLIANCE_PORT="+strconv.Itoa(s.compliancePort),
		"PAYMENT_PORT="+strconv.Itoa(s.paymentPort),
	)
	// Mount the entity's own PKI dir (its CA from gen-tls) over the repo default.
	if s.pkiDir != "" {
		env = append(env, "ENTITY_PKI_DIR="+s.pkiDir)
	}
	return env
}

// httpHealthy reports whether a GET to url returns 2xx within a short timeout.
func httpHealthy(ctx context.Context, url string) bool {
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

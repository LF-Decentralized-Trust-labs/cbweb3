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

// entityPKIVolumeKey is the FIXED local key backend-compose.yaml declares under
// its top-level volumes: section (see cb_tls there). Compose does not
// interpolate ${VAR} inside map keys, so the compose-local key must stay a
// literal string; per-spoke uniqueness comes from that entry's own
// name: ${SPOKE_ID}_cb_tls field instead. ENTITY_PKI_DIR is set to this literal
// key (not the real volume name) so Compose's short-syntax volume detection
// resolves it as a named-volume reference.
const entityPKIVolumeKey = "cb_tls"

// startBackendStackStep brings up an entity's 4 backend services (feature 034)
// from the per-entity backend compose template, then waits for the api-gateway
// health endpoint. Reused by the CB (found) and commercial banks (join). It reads
// the rendered per-entity .env and reaches the entity's dedicated infra by
// container name on the entity network.
type startBackendStackStep struct {
	name            string
	spokeID         string
	entityPrefix    string
	netName         string
	backendContext  string
	pkiDir          string
	useTLSVolume    bool
	envFile         string
	composePath     string
	bankCode        string
	paladinURL      string
	paladinIdentity string
	cactiURL        string
	imageTag        string
	// paymentOrchBesuRPCURL, when non-empty, un-gates the payment-orchestrator's
	// Besu-signing path (HTLC/fCeBM) by setting PAYMENT_ORCH_BESU_RPC_URL for the
	// compose. Empty leaves the path off (the template defaults BESU_RPC_URL empty).
	paymentOrchBesuRPCURL string
	apiPort               int
	authPort              int
	compliancePort        int
	paymentPort           int
	healthTimeout         time.Duration
	healthInterval        time.Duration
}

func newStartBackendStackStep(name string, p backendStackParams) Step {
	return &startBackendStackStep{
		name:                  name,
		spokeID:               p.SpokeID,
		entityPrefix:          p.EntityPrefix,
		netName:               p.NetName,
		backendContext:        p.BackendContext,
		pkiDir:                p.PKIDir,
		useTLSVolume:          p.UseTLSVolume,
		envFile:               p.EnvFile,
		composePath:           p.ComposePath,
		bankCode:              p.BankCode,
		paladinURL:            p.PaladinURL,
		paladinIdentity:       p.PaladinIdentity,
		cactiURL:              p.CactiURL,
		imageTag:              p.ImageTag,
		paymentOrchBesuRPCURL: p.PaymentOrchBesuRPCURL,
		apiPort:               p.APIPort,
		authPort:              p.AuthPort,
		compliancePort:        p.CompliancePort,
		paymentPort:           p.PaymentPort,
		healthTimeout:         p.HealthTimeout,
		healthInterval:        p.HealthInterval,
	}
}

// backendStackParams groups the (many) inputs to keep the constructor readable.
type backendStackParams struct {
	SpokeID               string
	EntityPrefix          string
	NetName               string
	BackendContext        string
	PKIDir                string
	UseTLSVolume          bool
	EnvFile               string
	ComposePath           string
	BankCode              string
	PaladinURL            string
	PaladinIdentity       string
	CactiURL              string
	ImageTag              string
	PaymentOrchBesuRPCURL string
	APIPort               int
	AuthPort              int
	CompliancePort        int
	PaymentPort           int
	HealthTimeout         time.Duration
	HealthInterval        time.Duration
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
	// --build forces the backend images to be rebuilt from current source: without it,
	// `up -d` reuses a stale image tag from a prior deploy and silently ships old binaries
	// (this bit the payment-orchestrator when its on-chain FX code changed).
	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", s.entityPrefix+"-backend", "-f", s.composePath, "up", "-d", "--build")
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
		"SPOKE_ID="+s.spokeID,
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
	// Mount the entity's own PKI over the repo default: a bind mount for banks
	// (pkiDir, a host path — gen-csr's async KYC flow still needs host access), or
	// a named Docker volume for the CB (useTLSVolume — cb_tls, seeded by gen-tls via
	// engine/dockervolume; see specs/026-tk4-compose-central-bank/plan.md addendum).
	// entity-backend/backend-compose.yaml's ${ENTITY_PKI_DIR}:/workspace/... mount
	// resolves to a bind mount or a named-volume reference based on this value.
	switch {
	case s.pkiDir != "":
		env = append(env, "ENTITY_PKI_DIR="+s.pkiDir)
	case s.useTLSVolume:
		env = append(env, "ENTITY_PKI_DIR="+entityPKIVolumeKey)
	}
	// The service images are non-root by default (uid 10001, finding R2-M-12), but the
	// three services that mount the PKI read the CA and persist issued certificates
	// there — and that path is owned by whoever ran this toolkit, at mode 0700 for a
	// bank. Any other uid can neither read nor write it, so those containers run as the
	// invoking user. Same reasoning as HOST_UID for the Besu containers (ADR-001 / T028);
	// the compose default keeps a hand-run stack on the image's non-root uid.
	env = append(env,
		"ENTITY_RUN_UID="+strconv.Itoa(os.Getuid()),
		"ENTITY_RUN_GID="+strconv.Itoa(os.Getgid()),
	)
	// Un-gate the payment-orchestrator Besu path only when a signing URL is set
	// (local). The compose defaults BESU_RPC_URL empty via ${PAYMENT_ORCH_BESU_RPC_URL-},
	// so leaving this unset preserves the disabled-by-default behaviour.
	if s.paymentOrchBesuRPCURL != "" {
		env = append(env, "PAYMENT_ORCH_BESU_RPC_URL="+s.paymentOrchBesuRPCURL)
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

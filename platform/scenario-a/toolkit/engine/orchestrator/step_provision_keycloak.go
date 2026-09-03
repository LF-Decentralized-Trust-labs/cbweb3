// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// provisionKeycloakStep brings up the entity's Keycloak (feature 034), backed by
// the entity's dedicated Postgres, importing the realm JSONs rendered from the
// per-entity plan. The CB instance hosts the central-bank + cbweb3 (NOC) realms.
//
// The rendered realm JSONs are seeded directly into the named volume
// ${entityPrefix}_keycloak_import (engine/dockervolume) — no host filesystem
// involved (deviation from the original SPOKE_DATA_DIR bind-mount design; see
// specs/026-tk4-compose-central-bank/plan.md addendum).
type provisionKeycloakStep struct {
	name          string
	entityPrefix  string
	netName       string
	composePath   string
	kcDBURL       string
	kcUser        string
	kcPassword    string
	kcAdminPass   string
	keycloakImage string
	hostPort      int
	realms        []KeycloakRealmPlan
	timeout       time.Duration
}

func newProvisionKeycloakStep(name string, p keycloakStepParams) Step {
	return &provisionKeycloakStep{
		name:          name,
		entityPrefix:  p.EntityPrefix,
		netName:       p.NetName,
		composePath:   p.ComposePath,
		kcDBURL:       p.KCDBURL,
		kcUser:        p.KCUser,
		kcPassword:    p.KCPassword,
		kcAdminPass:   p.KCAdminPassword,
		keycloakImage: p.KeycloakImage,
		hostPort:      p.HostPort,
		realms:        p.Realms,
		timeout:       p.Timeout,
	}
}

// keycloakStepParams groups the inputs for provisionKeycloakStep.
type keycloakStepParams struct {
	EntityPrefix    string
	NetName         string
	ComposePath     string
	KCDBURL         string
	KCUser          string
	KCPassword      string
	KeycloakImage   string
	HostPort        int
	Realms          []KeycloakRealmPlan
	Timeout         time.Duration
	KCAdminPassword string
}

func (s *provisionKeycloakStep) Name() string { return s.name }

func (s *provisionKeycloakStep) Check(ctx context.Context) (bool, error) {
	return httpHealthy(ctx, s.readyURL()), nil
}

// readyURL is the Keycloak readiness probe: a 200 on /realms/master means the
// master realm is served and the server has finished booting (and importing).
func (s *provisionKeycloakStep) readyURL() string {
	return fmt.Sprintf("http://localhost:%d/realms/master", s.hostPort)
}

func (s *provisionKeycloakStep) Run(ctx context.Context) error {
	// Seed the realm import JSONs the Keycloak container imports on startup
	// directly into the named volume — no host filesystem involved.
	for _, plan := range s.realms {
		data, err := renderRealmJSON(plan)
		if err != nil {
			return fmt.Errorf("render realm %s: %w", plan.Realm, err)
		}
		name := plan.Realm + "-realm.json"
		if err := writeVolumeFile(ctx, s.importVolume(), name, data, "0644"); err != nil {
			return fmt.Errorf("write realm %s to volume %s: %w", plan.Realm, s.importVolume(), err)
		}
	}

	// No --wait: the container healthcheck is for ops/NOC visibility. The step
	// gates on a Go-side HTTP poll (consistent with the backend step) so it does
	// not depend on the in-container shell's healthcheck quirks.
	// Unique compose project per entity (shared template would otherwise reconcile
	// and remove another entity's containers — see step_start_infra.go).
	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", s.entityPrefix+"-keycloak", "-f", s.composePath, "up", "-d")
	cmd.Env = s.composeEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("compose up keycloak: %w\noutput:\n%s", err, out)
	}

	deadline := time.Now().Add(s.timeout)
	for time.Now().Before(deadline) {
		if httpHealthy(ctx, s.readyURL()) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("keycloak readiness check timed out after %s", s.timeout)
}

// importVolume is the named volume holding the rendered realm JSONs, mounted
// read-only by the keycloak-compose.yaml template at /opt/keycloak/data/import.
func (s *provisionKeycloakStep) importVolume() string { return s.entityPrefix + "_keycloak_import" }

func (s *provisionKeycloakStep) composeEnv() []string {
	user := s.kcUser
	if user == "" {
		user = "default"
	}
	// No "default" fallback: Keycloak connects to the entity's Postgres, which is now
	// initialised with a generated password. A fallback here is how this step spent a
	// deploy authenticating as default/default against a database that had moved on —
	// it fails as a five-minute readiness timeout, which reads like a slow container
	// rather than a wrong credential.
	pass := s.kcPassword
	img := s.keycloakImage
	if img == "" {
		img = "quay.io/keycloak/keycloak:26.0"
	}
	return append(os.Environ(),
		"ENTITY_INFRA_PREFIX="+s.entityPrefix,
		"ENTITY_NET_NAME="+s.netName,
		"KEYCLOAK_HOST_PORT="+strconv.Itoa(s.hostPort),
		"KC_DB_URL="+s.kcDBURL,
		"KC_DB_USERNAME="+user,
		"KC_DB_PASSWORD="+pass,
		"KC_BOOTSTRAP_ADMIN_PASSWORD="+s.kcAdminPass,
		"KEYCLOAK_IMAGE="+img,
	)
}

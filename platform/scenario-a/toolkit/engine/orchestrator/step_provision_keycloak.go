// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
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
	// healthy is the readiness probe, injected so the Check can be tested without a server.
	// Defaults to httpHealthy.
	healthy func(ctx context.Context, url string) bool
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
		healthy:       httpHealthy,
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

// Check gates on the realms this step exists to create, not on the server being up.
//
// It used to probe /realms/master. That realm exists in every Keycloak, so the answer was
// "satisfied" whenever the container was running — and on an entity that had already been
// provisioned once, the step was skipped outright. A realm newly declared in the manifest
// therefore never got imported, and the apply reported success. The same shape as the defect
// reconcile-admin-users exists for, one level up: there it was a role that never reached an
// upgraded entity, here it is a whole realm.
//
// Every declared realm must answer. One missing is the upgrade case — an entity provisioned
// before a second realm was declared — and reporting satisfied there is what leaves it
// permanently absent.
//
// Note what this does NOT catch, so nobody reads more into a green step than it says: a realm
// that EXISTS but has drifted from the manifest. `--import-realm` skips a realm that is already
// there, so a changed origin or token lifespan needs converging, not importing. That is
// reconcile-keycloak-realm's job.
func (s *provisionKeycloakStep) Check(ctx context.Context) (bool, error) {
	probe := s.healthy
	if probe == nil {
		probe = httpHealthy
	}
	// Nothing declared: fall back to proving the server is up rather than answering satisfied
	// after checking nothing.
	if len(s.realms) == 0 {
		return probe(ctx, s.realmURL("master")), nil
	}
	for _, plan := range s.realms {
		if !probe(ctx, s.realmURL(plan.Realm)) {
			return false, nil
		}
	}
	return true, nil
}

// realmURL is the public endpoint of one realm. A 200 means Keycloak serves it, which for a
// realm the toolkit declares means the import landed.
func (s *provisionKeycloakStep) realmURL(realm string) string {
	return fmt.Sprintf("http://localhost:%d/realms/%s", s.hostPort, realm)
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

	// Wait on the same condition the Check gates on — every declared realm served — rather
	// than on the server answering at all. Waiting on master made this Run report success the
	// moment the container booted, whether or not the import it had just seeded produced
	// anything, which is the other half of the defect described above the Check.
	deadline := time.Now().Add(s.timeout)
	for time.Now().Before(deadline) {
		if ok, err := s.Check(ctx); err == nil && ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	// Name what is missing. "Keycloak timed out" reads as a slow container; "realm X is not
	// served" points at the import, which is where the fault actually is when a realm JSON is
	// malformed or the volume did not mount.
	return fmt.Errorf("keycloak did not serve %s within %s: the container is up but the realm "+
		"import did not produce them (check the container log for import errors, and that %s is mounted)",
		strings.Join(s.declaredRealms(), ", "), s.timeout, s.importVolume())
}

// declaredRealms names the realms this step is responsible for, for error messages.
func (s *provisionKeycloakStep) declaredRealms() []string {
	if len(s.realms) == 0 {
		return []string{"master"}
	}
	names := make([]string, 0, len(s.realms))
	for _, plan := range s.realms {
		names = append(names, plan.Realm)
	}
	return names
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

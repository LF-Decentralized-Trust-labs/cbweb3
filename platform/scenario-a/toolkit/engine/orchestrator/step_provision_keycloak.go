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
	// importGrace is how long the Run waits for a container to produce a declared realm on its
	// own before creating it with kcadm. A container that has just started runs the import
	// itself, and a slow one must not be misread as one that skipped it.
	importGrace time.Duration
	// pollInterval is the readiness poll period. Zero means the default.
	pollInterval time.Duration
	// healthy is the readiness probe, injected so the Check can be tested without a server.
	// Defaults to httpHealthy.
	healthy func(ctx context.Context, url string) bool
	// composeUp, dockerExecCmd and writeFile are this step's three Docker seams, injected so the
	// Run can be tested without a daemon. Nil means the real one.
	composeUp     func(ctx context.Context) error
	dockerExecCmd func(ctx context.Context, container, script string) ([]byte, error)
	writeFile     func(ctx context.Context, volume, filePath string, content []byte, mode string) error
}

// realmImportGrace is how long a declared realm gets to appear from the container's own startup
// import before the Run creates it with kcadm. Keycloak runs --import-realm before it starts
// serving HTTP, so on a first apply the realms are already there by the time the port answers
// and this grace costs nothing; it exists so a version that ordered those differently would not
// have its in-flight import overtaken by a create.
const realmImportGrace = 30 * time.Second

// keycloakReadyPoll is the readiness poll period.
const keycloakReadyPoll = 2 * time.Second

func newProvisionKeycloakStep(name string, p keycloakStepParams) Step {
	s := &provisionKeycloakStep{
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
		importGrace:   realmImportGrace,
		pollInterval:  keycloakReadyPoll,
		healthy:       httpHealthy,
	}
	s.composeUp = s.dockerComposeUp
	s.dockerExecCmd = dockerExecScript
	s.writeFile = writeVolumeFile
	return s
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
	write := s.writeFile
	if write == nil {
		write = writeVolumeFile
	}
	up := s.composeUp
	if up == nil {
		up = s.dockerComposeUp
	}
	interval := s.pollInterval
	if interval <= 0 {
		interval = keycloakReadyPoll
	}

	// Seed the realm import JSONs the Keycloak container imports on startup
	// directly into the named volume — no host filesystem involved. The same files are what
	// createMissingRealms feeds to kcadm, so a realm created after the fact carries exactly
	// what the import would have applied.
	for _, plan := range s.realms {
		data, err := renderRealmJSON(plan)
		if err != nil {
			return fmt.Errorf("render realm %s: %w", plan.Realm, err)
		}
		name := plan.Realm + "-realm.json"
		if err := write(ctx, s.importVolume(), name, data, "0644"); err != nil {
			return fmt.Errorf("write realm %s to volume %s: %w", plan.Realm, s.importVolume(), err)
		}
	}

	if err := up(ctx); err != nil {
		return err
	}

	// Wait on the same condition the Check gates on — every declared realm served — rather
	// than on the server answering at all. Waiting on master made this Run report success the
	// moment the container booted, whether or not the import it had just seeded produced
	// anything, which is the other half of the defect described above the Check.
	//
	// Waiting alone is not enough either, and this is where being unsatisfied stops being
	// sufficient: `--import-realm` runs at container startup and skips a realm that already
	// exists, so `up -d` against an already-running container whose compose config did not
	// change is a no-op — the seeded JSON is never read. On an entity whose Keycloak predates
	// the declaration the realm can only come from kcadm, which is what createMissingRealms
	// does once the server answers.
	deadline := time.Now().Add(s.timeout)
	// The grace is a courtesy to a container that is still running its own import, never a
	// reason to spend the whole budget waiting for something only kcadm can produce. Clamped,
	// so a short configured timeout does not silently disable the create.
	grace := s.importGrace
	if limit := s.timeout / 3; grace > limit {
		grace = limit
	}
	graceOver := time.Now().Add(grace)
	created := false
	for time.Now().Before(deadline) {
		if ok, err := s.Check(ctx); err == nil && ok {
			return nil
		}
		// Only after the grace, and only once the server answers at all: a container still
		// running its own startup import must not be overtaken, and one still booting must not
		// be misread as one that skipped the import — kcadm cannot log in to either, and
		// turning a slow boot into a failed create trades a wait for a wrong error.
		//
		// One attempt, so a create that keeps failing surfaces its own error instead of being
		// retried until the generic timeout hides it.
		if !created && !time.Now().Before(graceOver) && s.serverUp(ctx) {
			if err := s.createMissingRealms(ctx); err != nil {
				return err
			}
			created = true
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
	// Name what is missing and the cause an operator cannot find in the container log. "Keycloak
	// timed out" reads as a slow container, and the log of a container that skipped an import
	// shows no error at all.
	return fmt.Errorf("keycloak did not serve %s within %s: the realm import runs only at container "+
		"startup and skips a realm that already exists, so a realm declared after this container was "+
		"created cannot come from waiting — this step creates it with kcadm instead, so check the "+
		"container log for a kcadm/import error and that %s is mounted",
		strings.Join(s.declaredRealms(), ", "), s.timeout, s.importVolume())
}

// dockerComposeUp brings the entity's Keycloak up.
//
// No --wait: the container healthcheck is for ops/NOC visibility. The step gates on a Go-side
// HTTP poll (consistent with the backend step) so it does not depend on the in-container shell's
// healthcheck quirks. Unique compose project per entity (a shared one would otherwise reconcile
// and remove another entity's containers — see step_start_infra.go).
func (s *provisionKeycloakStep) dockerComposeUp(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-p", s.entityPrefix+"-keycloak", "-f", s.composePath, "up", "-d")
	cmd.Env = s.composeEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("compose up keycloak: %w\noutput:\n%s", err, out)
	}
	return nil
}

// container is the entity's Keycloak container, named by the compose template.
func (s *provisionKeycloakStep) container() string { return s.entityPrefix + "-keycloak" }

// serverUp reports whether Keycloak answers at all. master is the right probe HERE — unlike in
// the Check, where gating on it was the defect: this asks "can kcadm log in yet", not "is the
// entity's realm there".
func (s *provisionKeycloakStep) serverUp(ctx context.Context) bool {
	probe := s.healthy
	if probe == nil {
		probe = httpHealthy
	}
	return probe(ctx, s.realmURL("master"))
}

// createMissingRealms creates, through kcadm, every declared realm Keycloak does not serve.
//
// kcadm is the only writer that can add a realm to a running Keycloak. The file it reads is the
// realm JSON this step seeded into the import volume, which the compose template mounts read-only
// at keycloakImportDir — the same document `--import-realm` would have applied, so a realm
// created here and one imported at first boot carry the same clients, roles and users.
//
// Scenario B never had this defect because it never relied on --import-realm: it creates its
// realms with kcadm (step_found_spoke.go, step_join.go). This is the same approach, not shared
// code — the realms and their contents differ.
func (s *provisionKeycloakStep) createMissingRealms(ctx context.Context) error {
	run := s.dockerExecCmd
	if run == nil {
		run = dockerExecScript
	}
	probe := s.healthy
	if probe == nil {
		probe = httpHealthy
	}
	for _, plan := range s.realms {
		if probe(ctx, s.realmURL(plan.Realm)) {
			continue
		}
		out, err := run(ctx, s.container(), realmCreateScript(keycloakAdminCLI, plan.Realm))
		if err != nil {
			return fmt.Errorf("create realm %s from %s: %w\noutput:\n%s",
				plan.Realm, realmImportFile(plan.Realm), err, out)
		}
	}
	return nil
}

// keycloakImportDir is where keycloak-compose.yaml mounts the import volume inside the container.
// Read-only, which is all kcadm needs.
const keycloakImportDir = "/opt/keycloak/data/import"

// realmImportFile is the in-container path of the realm JSON this step seeded.
func realmImportFile(realm string) string {
	return keycloakImportDir + "/" + realm + "-realm.json"
}

// realmCreateScript creates one realm from its seeded JSON.
//
// It carries a PATH, not a secret: the realm document — with its client secrets and its users'
// passwords — is already inside the container, in the volume the compose template mounts, and the
// login is kcadmLogin, which reads the admin password from the container's own environment. A
// guard in keycloak_realm_create_test.go fails if either changes.
func realmCreateScript(kc, realm string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s || exit 1\n", kcadmLogin(kc))
	fmt.Fprintf(&b, "%s create realms -f %s || exit 1\n", kc, realmImportFile(realm))
	return b.String()
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

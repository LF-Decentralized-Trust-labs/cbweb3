// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// errRealmCreateFailed is the kcadm failure the failure case injects.
var errRealmCreateFailed = errors.New("exit status 1")

// The half of the readiness defect that gating alone does not close.
//
// Gating provision-keycloak on every declared realm (keycloak_realm_readiness_test.go) stops the
// apply from reporting success over a realm that was never applied. It does not produce the
// realm. `--import-realm` runs at container startup and skips a realm that already exists, so on
// an entity whose Keycloak container predates the declaration the Run seeds the JSON into the
// import volume, `docker compose up -d` leaves the already-running container untouched, and the
// step polls until its timeout.
//
// That outcome is worse than it first reads: a failed step aborts the whole apply
// (engine/apply/apply.go), so reconcile-admin-users, reconcile-keycloak-realm, the backend, the
// frontend and register-relay stop running too — on this apply and on every later one, until an
// operator removes the Keycloak container by hand. Being unsatisfied is necessary, not
// sufficient.
//
// The same trap is reachable without any upgrade: any path that leaves the container up with one
// realm missing (a malformed realm JSON that Keycloak logs and serves past, for instance) is
// permanently stuck, because waiting can never run an import again.
//
// So the step creates the missing realm with kcadm, the only writer that can add a realm to a
// Keycloak that is already running. Scenario B never had this shape — it creates its realms with
// kcadm rather than with --import-realm (step_found_spoke.go:437, step_join.go:169).

// fakeKeycloak stands in for the container: which realms it serves, what was written into the
// import volume, and every script that reached it through docker exec.
type fakeKeycloak struct {
	served map[string]bool
	// createServes makes a successful create script start serving its realm, which is what a
	// real Keycloak does. Off for the failure cases.
	createServes bool
	execErr      error
	scripts      []string
	written      []string
	upCalls      int
}

// createStep builds the step with all three Docker seams faked, so Run is exercised without a
// daemon.
func createStep(t *testing.T, kc *fakeKeycloak, plans []KeycloakRealmPlan, timeout time.Duration) *provisionKeycloakStep {
	t.Helper()
	if kc.served == nil {
		kc.served = map[string]bool{}
	}
	s := &provisionKeycloakStep{
		name: StepProvisionKeycloak, entityPrefix: "cb", hostPort: 8081, realms: plans,
		kcAdminPass: "s3cr3t-admin-pw", timeout: timeout,
		// No grace and a 1ms poll: the grace exists so a container that is still running its
		// own import is not misread as one that skipped it, and it is not what these cover.
		importGrace: 0, pollInterval: time.Millisecond,
		healthy: func(_ context.Context, url string) bool {
			for realm, ok := range kc.served {
				if ok && strings.HasSuffix(url, "/realms/"+realm) {
					return true
				}
			}
			return false
		},
		composeUp: func(context.Context) error { kc.upCalls++; return nil },
		writeFile: func(_ context.Context, _, name string, _ []byte, _ string) error {
			kc.written = append(kc.written, name)
			return nil
		},
	}
	s.dockerExecCmd = func(_ context.Context, _, script string) ([]byte, error) {
		kc.scripts = append(kc.scripts, script)
		if kc.execErr != nil {
			return []byte("kcadm: conflict"), kc.execErr
		}
		if kc.createServes {
			for _, plan := range plans {
				if strings.Contains(script, realmImportFile(plan.Realm)) {
					kc.served[plan.Realm] = true
				}
			}
		}
		return nil, nil
	}
	return s
}

// The load-bearing case: the container is up (master answers), the entity's second realm was
// declared after that container was created, and the import will never run again for it.
func TestProvisionKeycloak_CreatesTheRealmTheImportSkipped(t *testing.T) {
	plans := realmPlans("cb-realm", "noc")
	kc := &fakeKeycloak{served: map[string]bool{"master": true, "cb-realm": true}, createServes: true}
	s := createStep(t, kc, plans, 2*time.Second)

	if err := s.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !kc.served["noc"] {
		t.Fatal("the declared realm is still absent after a successful Run; --import-realm does " +
			"not reapply, so waiting for a running container to produce it never ends")
	}
	created := ""
	for _, script := range kc.scripts {
		if strings.Contains(script, "create realms") {
			created = script
		}
	}
	if created == "" {
		t.Fatalf("no realm was created through kcadm (scripts: %v); the step would have polled "+
			"until its timeout and failed the whole apply with the realm still missing", kc.scripts)
	}
	if !strings.Contains(created, realmImportFile("noc")) {
		t.Errorf("the create script does not read the seeded realm JSON %q: %s",
			realmImportFile("noc"), created)
	}
	if strings.Contains(created, realmImportFile("cb-realm")) {
		t.Error("the create script names a realm that is already served; recreating one is a " +
			"conflict, and the step must only create what is missing")
	}
}

// The steady state must stay free: an entity whose realms are all served has nothing to create,
// and reaching for kcadm there would rewrite a realm on every apply.
func TestProvisionKeycloak_ServedRealmsAreNotRecreated(t *testing.T) {
	plans := realmPlans("cb-realm", "noc")
	kc := &fakeKeycloak{served: map[string]bool{"master": true, "cb-realm": true, "noc": true}}
	s := createStep(t, kc, plans, time.Second)

	if err := s.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(kc.scripts) != 0 {
		t.Errorf("the Run executed %d script(s) against a realm set that already matched: %v",
			len(kc.scripts), kc.scripts)
	}
}

// A container that is still booting must not be mistaken for one that skipped its import. kcadm
// cannot log in before the server answers, and turning a slow boot into a failed create would
// trade a five-minute wait for an immediate, wrong error.
func TestProvisionKeycloak_DoesNotReachForKcadmBeforeTheServerAnswers(t *testing.T) {
	plans := realmPlans("cb-realm")
	kc := &fakeKeycloak{} // nothing served at all: not even master
	s := createStep(t, kc, plans, 20*time.Millisecond)

	err := s.Run(context.Background())
	if err == nil {
		t.Fatal("a Keycloak that never answers must fail the step")
	}
	if len(kc.scripts) != 0 {
		t.Errorf("the Run ran kcadm against a container that answers nothing: %v", kc.scripts)
	}
}

// The timeout message is the operator's only pointer. It used to name two causes that are not the
// one that applies here (import errors, an unmounted volume), and stayed silent about the one
// that is: the import runs only at startup.
func TestProvisionKeycloak_TimeoutNamesTheStartupOnlyImport(t *testing.T) {
	plans := realmPlans("cb-realm")
	kc := &fakeKeycloak{served: map[string]bool{"master": true}} // create does not help
	s := createStep(t, kc, plans, 20*time.Millisecond)

	err := s.Run(context.Background())
	if err == nil {
		t.Fatal("a realm that never appears must fail the step")
	}
	if !strings.Contains(err.Error(), "cb-realm") {
		t.Errorf("the message does not name the missing realm: %v", err)
	}
	if !strings.Contains(err.Error(), "startup") {
		t.Errorf("the message does not say the import runs only at container startup, which is "+
			"the cause an operator cannot find in the container log: %v", err)
	}
}

// The grace must never be able to eat the whole budget. It is configurable per entity
// (deps.Timeouts), so a timeout shorter than realmImportGrace would leave the create silently
// unreachable — the exact silence this whole change exists to remove.
func TestProvisionKeycloak_ShortTimeoutStillCreatesTheRealm(t *testing.T) {
	plans := realmPlans("noc")
	kc := &fakeKeycloak{served: map[string]bool{"master": true}, createServes: true}
	s := createStep(t, kc, plans, 30*time.Millisecond)
	s.importGrace = realmImportGrace // the production value, far beyond the timeout

	if err := s.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(kc.scripts) == 0 {
		t.Error("a timeout shorter than the import grace disabled the realm create entirely; the " +
			"grace must be clamped to a fraction of the budget, not applied whole")
	}
}

// A create that fails must say which realm and from which file. Swallowing it would leave the
// step reporting the generic timeout, which points at the wrong lever.
func TestProvisionKeycloak_CreateFailureNamesTheRealmAndTheFile(t *testing.T) {
	plans := realmPlans("noc")
	kc := &fakeKeycloak{served: map[string]bool{"master": true}, execErr: errRealmCreateFailed}
	s := createStep(t, kc, plans, time.Second)

	err := s.Run(context.Background())
	if err == nil {
		t.Fatal("a failed realm create must fail the step")
	}
	if !strings.Contains(err.Error(), "noc") || !strings.Contains(err.Error(), realmImportFile("noc")) {
		t.Errorf("the error names neither the realm nor the file it read: %v", err)
	}
	if !strings.Contains(err.Error(), "kcadm: conflict") {
		t.Errorf("kcadm's own output is not in the error, so the operator cannot see why: %v", err)
	}
}

// Same rule as every other kcadm script in this package: the admin secret is read from the
// container's own environment, never interpolated into an argv three process lists can see.
func TestProvisionKeycloak_RealmCreateNeverPutsTheAdminSecretInArgv(t *testing.T) {
	plans := realmPlans("noc")
	kc := &fakeKeycloak{served: map[string]bool{"master": true}, createServes: true}
	s := createStep(t, kc, plans, time.Second)

	if err := s.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(kc.scripts) == 0 {
		t.Fatal("no script ran, so this assertion would be vacuous")
	}
	for _, script := range kc.scripts {
		if strings.Contains(script, s.kcAdminPass) {
			t.Errorf("the admin password value is embedded in a script: %s", script)
		}
		if strings.Contains(script, "--password") {
			t.Errorf("the script passes the admin secret as --password, which lands in the "+
				"container's process list, the host's and the toolkit's: %s", script)
		}
	}
}

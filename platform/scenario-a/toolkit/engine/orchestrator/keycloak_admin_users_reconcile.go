// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// Operator accounts and their realm roles reach Keycloak through the realm JSON that
// provision-keycloak seeds into the import volume — and Keycloak imports a realm only when it does
// not already exist. On top of that, the step's Check is an HTTP readiness probe, so it skips
// whenever Keycloak is up at all.
//
// Both together mean a role newly declared in spec.adminUsers never reaches an entity that is
// already provisioned: the step does not run, and even if it did, the import would not overwrite
// the live realm.
//
// Spec 042 makes that concrete. It re-gates POST /approve-kyc to ROLE_ADMISSION, a role granted only
// to an admin user declared as ADMISSION. On a fresh deploy the manifest carries it. On an existing
// stack, adding the user and re-applying creates neither the realm role nor the user — so the route
// closes on a role nobody holds and KYC approval becomes unreachable, with nothing in the apply
// report saying so.
//
// This step converges that on every run, through kcadm inside the running container, which is the
// only writer that can change a realm Keycloak has already imported. Scenario B reached the same
// conclusion one release earlier for the NOC client's origins (reconcile-noc-origins): "already
// provisioned" must not be read as "already correct".
//
// It converges towards the manifest and does NOT delete users or revoke roles it did not declare.
// An operator account created by hand during an incident is not something an apply should silently
// remove; removing accounts is a separate decision.

// keycloakAdminCLI is kcadm inside the Keycloak container image.
const keycloakAdminCLI = "/opt/keycloak/bin/kcadm.sh"

// adminUsersReadScript reports which realm roles each declared user currently holds, one block per
// user, so a user that does not exist is distinguishable from one that exists without its roles.
func adminUsersReadScript(kc, realm, adminPassword string, users []KeycloakUserPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%[1]s config credentials --server http://localhost:8080 --realm master --user admin --password %[2]s >/dev/null 2>&1 || exit 1\n",
		kc, adminPassword)
	for _, u := range users {
		// `|| true` per user: a missing user makes get-roles exit non-zero, and that is an answer
		// — it holds nothing — not a reason to abandon the sweep.
		fmt.Fprintf(&b, "echo \"USER %[1]s\"; %[2]s get-roles -r %[3]s --uusername %[1]s --fields name 2>/dev/null || true\n",
			u.Username, kc, realm)
	}
	return b.String()
}

// adminUsersReconcileScript creates any missing realm role, user, password and grant. Every command
// tolerates "already exists", so it is safe to run on a realm that is already correct.
func adminUsersReconcileScript(kc, realm, adminPassword string, users []KeycloakUserPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%[1]s config credentials --server http://localhost:8080 --realm master --user admin --password %[2]s || exit 1\n",
		kc, adminPassword)
	for _, r := range declaredRealmRoles(users) {
		fmt.Fprintf(&b, "%[1]s create roles -r %[2]s -s name=%[3]s || true\n", kc, realm, r)
	}
	for _, u := range users {
		// emailVerified/firstName/lastName are required by Keycloak 26's declarative user profile
		// for the password grant to work, exactly as the realm import sets them.
		fmt.Fprintf(&b, "%[1]s create users -r %[2]s -s username=%[3]s -s enabled=true -s emailVerified=true -s email=%[3]s -s firstName=%[3]s -s lastName=Operator || true\n",
			kc, realm, u.Username)
		fmt.Fprintf(&b, "%[1]s set-password -r %[2]s --username %[3]s --new-password %[4]s || true\n",
			kc, realm, u.Username, u.Password)
		for _, r := range u.Roles {
			fmt.Fprintf(&b, "%[1]s add-roles -r %[2]s --uusername %[3]s --rolename %[4]s || true\n",
				kc, realm, u.Username, r)
		}
	}
	return b.String()
}

// parseAdminUserRoles turns the read script's output into username → set of realm roles held. It
// reads the `"name" : "ROLE_X"` lines between USER markers rather than parsing JSON, because kcadm
// prints one array per invocation and the markers are what tie a block to its user.
func parseAdminUserRoles(out string) map[string]map[string]bool {
	held := map[string]map[string]bool{}
	current := ""
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "USER ") {
			current = strings.TrimSpace(strings.TrimPrefix(line, "USER "))
			if _, ok := held[current]; !ok {
				held[current] = map[string]bool{}
			}
			continue
		}
		if current == "" || !strings.Contains(line, "\"name\"") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 2 {
			continue
		}
		value := strings.Trim(strings.TrimSuffix(strings.TrimSpace(parts[len(parts)-1]), ","), "\"")
		if value != "" {
			held[current][value] = true
		}
	}
	return held
}

// adminUsersSatisfied reports whether every declared user holds every role the manifest gives it.
// A missing user and a missing grant are the same answer — converge.
func adminUsersSatisfied(out string, users []KeycloakUserPlan) bool {
	held := parseAdminUserRoles(out)
	for _, u := range users {
		got, ok := held[u.Username]
		if !ok {
			return false
		}
		for _, want := range u.Roles {
			if !got[want] {
				return false
			}
		}
	}
	return true
}

// declaredRealmRoles is the sorted set of realm roles the declared users require.
func declaredRealmRoles(users []KeycloakUserPlan) []string {
	seen := map[string]bool{}
	for _, u := range users {
		for _, r := range u.Roles {
			seen[r] = true
		}
	}
	out := make([]string, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// usersFromRealmPlans flattens the realms' declared users. Realms are provisioned together and share
// one Keycloak, so they reconcile together.
func usersFromRealmPlans(plans []KeycloakRealmPlan) map[string][]KeycloakUserPlan {
	byRealm := map[string][]KeycloakUserPlan{}
	for _, p := range plans {
		if len(p.Users) > 0 {
			byRealm[p.Realm] = append(byRealm[p.Realm], p.Users...)
		}
	}
	return byRealm
}

// reconcileAdminUsersStep converges the declared operator accounts on every run.
type reconcileAdminUsersStep struct {
	name          string
	entityPrefix  string
	kcAdminPass   string
	realms        []KeycloakRealmPlan
	dockerExecCmd func(ctx context.Context, container, script string) ([]byte, error)
}

func newReconcileAdminUsersStep(name, entityPrefix, kcAdminPass string, realms []KeycloakRealmPlan) Step {
	return &reconcileAdminUsersStep{
		name: name, entityPrefix: entityPrefix, kcAdminPass: kcAdminPass, realms: realms,
		dockerExecCmd: dockerExecScript,
	}
}

func (s *reconcileAdminUsersStep) Name() string { return s.name }

func (s *reconcileAdminUsersStep) container() string { return s.entityPrefix + "-keycloak" }

// Check reports satisfied only when every declared user in every realm already holds its roles. An
// unreachable Keycloak answers "not satisfied", so the Run converges — a provisioned entity with its
// containers down is ordinary, and skipping there is exactly when this must not skip.
func (s *reconcileAdminUsersStep) Check(ctx context.Context) (bool, error) {
	byRealm := usersFromRealmPlans(s.realms)
	if len(byRealm) == 0 {
		return true, nil
	}
	for realm, users := range byRealm {
		out, err := s.dockerExecCmd(ctx, s.container(),
			adminUsersReadScript(keycloakAdminCLI, realm, s.kcAdminPass, users))
		if err != nil {
			return false, nil
		}
		if !adminUsersSatisfied(string(out), users) {
			return false, nil
		}
	}
	return true, nil
}

func (s *reconcileAdminUsersStep) Run(ctx context.Context) error {
	for realm, users := range usersFromRealmPlans(s.realms) {
		if _, err := s.dockerExecCmd(ctx, s.container(),
			adminUsersReconcileScript(keycloakAdminCLI, realm, s.kcAdminPass, users)); err != nil {
			return fmt.Errorf("reconcile admin users in realm %s: %w", realm, err)
		}
	}
	return nil
}

func dockerExecScript(ctx context.Context, container, script string) ([]byte, error) {
	return exec.CommandContext(ctx, "docker", "exec", container, "bash", "-c", script).CombinedOutput()
}

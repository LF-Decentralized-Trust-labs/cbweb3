// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"os"
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

// kcadmLogin returns the `config credentials` command both scripts start with, authenticating
// WITHOUT putting the admin secret in any argv.
//
// The script is one argument to `docker exec … bash -c`, so a secret interpolated into it lands in
// three process lists: the container's (the shell's argv and the kcadm JVM's own), the host's (the
// script is an argument to the docker client) and the toolkit's. Passing it as --password exposed
// it in all three, on every apply — the read script runs from the step's Check, so even a converged
// entity that changes nothing paid the exposure.
//
// Nothing needs to be shipped in: the value is already inside the container, in Keycloak's own
// KC_BOOTSTRAP_ADMIN_PASSWORD, set by provisioning/templates/entity-keycloak/keycloak-compose.yaml.
// kcadm reads its password from KC_CLI_PASSWORD when the flag is absent.
//
// The assignment is a per-command prefix rather than a script-wide export so it reaches kcadm and
// nothing else, and so a script that drops this helper cannot keep authenticating by accident.
//
// Scenario B reached this first (engine/orchestrator/keycloak_provision.go) and verified it against
// quay.io/keycloak/keycloak:26.0, the image this scenario also pins: with the variable set and no
// flag, login succeeds; with it unset and no flag, kcadm fails with "Console is not active, but
// password is required" and exit 1 — so the `|| exit 1` here still breaks on a failed login.
func kcadmLogin(kc string) string {
	return fmt.Sprintf(
		`KC_CLI_PASSWORD="$KC_BOOTSTRAP_ADMIN_PASSWORD" %s config credentials `+
			`--server http://localhost:8080 --realm master --user admin`, kc)
}

// adminUsersReadScript reports which realm roles each declared user currently holds, one block per
// user, so a user that does not exist is distinguishable from one that exists without its roles.
func adminUsersReadScript(kc, realm string, users []KeycloakUserPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s >/dev/null 2>&1 || exit 1\n", kcadmLogin(kc))
	for _, u := range users {
		// `|| true` per user: a missing user makes get-roles exit non-zero, and that is an answer
		// — it holds nothing — not a reason to abandon the sweep.
		fmt.Fprintf(&b, "echo \"USER %[1]s\"; %[2]s get-roles -r %[3]s --uusername %[1]s --fields name 2>/dev/null || true\n",
			u.Username, kc, realm)
	}
	return b.String()
}

// operatorPasswordVar is the environment variable one operator's password travels in.
//
// Indexed rather than derived from the username: a username is an email address, and @ and . are
// not valid in a shell variable name. The index is the operator's position in the same slice the
// script is generated from, so the name in the script and the value in the environment cannot
// drift — one loop over one list produces both.
func operatorPasswordVar(i int) string { return fmt.Sprintf("KC_OP_PW_%d", i) }

// operatorPasswordEnv pairs each operator's variable with its value, as "NAME=value" for os/exec.
// Only the NAMES reach docker's argv — dockerExecArgs derives them from these pairs.
func operatorPasswordEnv(users []KeycloakUserPlan) []string {
	env := make([]string, 0, len(users))
	for i, u := range users {
		env = append(env, operatorPasswordVar(i)+"="+u.Password)
	}
	return env
}

// adminUsersReconcileScript creates any missing realm role, user, password and grant. Every command
// tolerates "already exists", so it is safe to run on a realm that is already correct.
func adminUsersReconcileScript(kc, realm string, users []KeycloakUserPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s || exit 1\n", kcadmLogin(kc))
	for _, r := range declaredRealmRoles(users) {
		fmt.Fprintf(&b, "%[1]s create roles -r %[2]s -s name=%[3]s || true\n", kc, realm, r)
	}
	for i, u := range users {
		// emailVerified/firstName/lastName are required by Keycloak 26's declarative user profile
		// for the password grant to work, exactly as the realm import sets them.
		fmt.Fprintf(&b, "%[1]s create users -r %[2]s -s username=%[3]s -s enabled=true -s emailVerified=true -s email=%[3]s -s firstName=%[3]s -s lastName=Operator || true\n",
			kc, realm, u.Username)
		// The password is NOT written here. `--new-password <value>` puts it in the kcadm JVM's
		// argv inside the container and, because this whole script is one argument to
		// `docker exec … bash -c`, in the docker client's argv on the host too — `ps` shows both
		// to any user on the machine. kcadm reads KC_CLI_PASSWORD when the flag is absent (its own
		// `set-password --help` says so), and a per-command prefix puts the value in that one
		// process's environment instead of anyone's argv.
		//
		// The value reaches the container through `docker exec -e <name>` (operatorPasswordEnv +
		// dockerExecArgs), the pass-through form, which names the variable without its value.
		//
		// Same rule as Scenario B's keycloak_operator_password.go — a deliberate copy, since the
		// two toolkits share no library; see docs/scenario-drift.md §14.
		fmt.Fprintf(&b, "KC_CLI_PASSWORD=\"$%[4]s\" %[1]s set-password -r %[2]s --username %[3]s || true\n",
			kc, realm, u.Username, operatorPasswordVar(i))
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
	name         string
	entityPrefix string
	// kcAdminPass is deliberately still held after the scripts stopped carrying it: the exposure
	// guard in keycloak_secret_exposure_test.go can only prove the secret is not embedded if the
	// step actually has one to embed. Removing it would make that assertion vacuous.
	kcAdminPass string
	realms      []KeycloakRealmPlan
	// dockerExecCmd runs script in container with env ("NAME=value" pairs) available to it. Only
	// the NAMES reach docker's argv; see dockerExecArgs.
	dockerExecCmd func(ctx context.Context, container, script string, env []string) ([]byte, error)
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
		// The read script sets no password, so it needs no operator secret in its environment.
		out, err := s.dockerExecCmd(ctx, s.container(),
			adminUsersReadScript(keycloakAdminCLI, realm, users), nil)
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
			adminUsersReconcileScript(keycloakAdminCLI, realm, users),
			operatorPasswordEnv(users)); err != nil {
			return fmt.Errorf("reconcile admin users in realm %s: %w", realm, err)
		}
	}
	return nil
}

func dockerExecScript(ctx context.Context, container, script string, env []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "docker", dockerExecArgs(container, script, env)...)
	// The values live here, in the child's environment, and nowhere else. os/exec passes an
	// environment through a private channel, not through the argument vector `ps` prints.
	cmd.Env = append(os.Environ(), env...)
	return cmd.CombinedOutput()
}

// dockerExecArgs builds the docker argument vector for one exec.
//
// Split out from dockerExecScript so the argument vector can be asserted directly: it is the
// artefact that ends up in `ps`, and a test that can only see the script would not notice a
// secret that moved from the script into an argument.
//
// `-e NAME` is the pass-through form: docker copies the value from its own environment. The more
// commonly shown `-e NAME=value` also works, and is exactly the mistake this guards against — it
// moves the secret from the script into the docker client's argv, one step to the left.
func dockerExecArgs(container, script string, env []string) []string {
	args := []string{"exec"}
	for _, pair := range env {
		name, _, found := strings.Cut(pair, "=")
		if !found || name == "" {
			continue
		}
		args = append(args, "-e", name)
	}
	return append(args, container, "bash", "-c", script)
}

// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// Ported from Scenario B (engine/orchestrator/keycloak_secret_exposure_test.go), where the same
// defect was found and fixed. The audit in docs/guard-parity.md confirmed A carried the defect the
// guard exists to prevent: reconcile-admin-users interpolated the Keycloak admin secret into
// `--password` on both its read and its write script.
//
// Why that matters: the whole script is a single argument to `docker exec … bash -c`, so a secret
// written into it lands in three process lists — the container's (the shell's argv and the kcadm
// JVM's own), the host's (the script is an argument to the docker client) and the toolkit's.
//
// Nothing needs to be shipped in. The value is already inside the container, in Keycloak's own
// KC_BOOTSTRAP_ADMIN_PASSWORD, set by provisioning/templates/entity-keycloak/keycloak-compose.yaml
// from the same variable this step holds. kcadm reads its password from KC_CLI_PASSWORD when the
// flag is absent — verified against quay.io/keycloak/keycloak:26.0, the image this scenario pins.
//
// Scenario A differs from B in where the exposure lives: B builds one provisioning script per mode,
// A builds two scripts in one step (Check reads, Run writes). Both are covered here — a Check that
// still leaks would expose the secret on every apply, including the ones that converge and change
// nothing.

// sentinelAdminPassword is the value the scripts would embed. It is passed in directly rather than
// read back from generated state, so a fix that stops interpolating anything cannot make this test
// vacuous by accident.
const sentinelAdminPassword = "s3ntinel-admin-secret-must-not-appear"

// sentinelOperatorPasswords are the operator secrets the step holds. Distinct per user so a test
// can tell which one leaked, and distinct from the admin secret so the two exposures cannot be
// confused for one another.
var sentinelOperatorUsers = []KeycloakUserPlan{
	{Username: "gov@cb.test", Password: "s3ntinel-operator-gov-must-not-appear", Roles: []string{"ROLE_GOVERNANCE"}},
	{Username: "adm@cb.test", Password: "s3ntinel-operator-adm-must-not-appear", Roles: []string{"ROLE_ADMISSION"}},
}

// execCall is one `docker exec` the step made: the script it handed to `bash -c`, and the
// environment it asked docker to forward.
type execCall struct {
	script string
	env    []string
}

// callsFromReconcileStep returns every exec the step made, across both Check and Run.
func callsFromReconcileStep(t *testing.T) map[string]execCall {
	t.Helper()
	calls := map[string]execCall{}
	newStep := func(record func(execCall)) *reconcileAdminUsersStep {
		return &reconcileAdminUsersStep{
			name: StepReconcileAdminUsers, entityPrefix: "cb", kcAdminPass: sentinelAdminPassword,
			realms: []KeycloakRealmPlan{{Realm: "cb-realm", Users: sentinelOperatorUsers}},
			dockerExecCmd: func(_ context.Context, _, script string, env []string) ([]byte, error) {
				record(execCall{script: script, env: env})
				return nil, nil
			},
		}
	}
	if _, err := newStep(func(c execCall) { calls["check"] = c }).Check(context.Background()); err != nil {
		t.Fatalf("check: %v", err)
	}
	if err := newStep(func(c execCall) { calls["run"] = c }).Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, phase := range []string{"check", "run"} {
		if calls[phase].script == "" {
			t.Fatalf("no script was recorded for %s; the step did not shell into Keycloak", phase)
		}
	}
	return calls
}

// scriptsFromReconcileStep is the script half of callsFromReconcileStep, for the assertions that
// only care about the text.
func scriptsFromReconcileStep(t *testing.T) map[string]string {
	t.Helper()
	scripts := map[string]string{}
	for phase, call := range callsFromReconcileStep(t) {
		scripts[phase] = call.script
	}
	return scripts
}

// TestKeycloakReconcile_NeverEmbedsTheAdminSecret is the load-bearing assertion: the secret must
// not appear anywhere in either generated script.
func TestKeycloakReconcile_NeverEmbedsTheAdminSecret(t *testing.T) {
	for phase, script := range scriptsFromReconcileStep(t) {
		if strings.Contains(script, sentinelAdminPassword) {
			t.Errorf("the %s script embeds the Keycloak admin secret.\n"+
				"That text is an argument to `docker exec`, so the value lands in the container's argv, "+
				"the host's argv and the toolkit's own. Reference the container's "+
				"KC_BOOTSTRAP_ADMIN_PASSWORD instead — the value is already there.", phase)
		}
	}
}

// TestKeycloakReconcile_AuthenticatesFromTheContainerEnvironment pins the shape, which the value
// check alone cannot: a script that stopped passing the flag but also stopped authenticating would
// satisfy the test above and fail on a real server.
func TestKeycloakReconcile_AuthenticatesFromTheContainerEnvironment(t *testing.T) {
	for phase, script := range scriptsFromReconcileStep(t) {
		if i := indexOfCredentialsWithPasswordFlag(script); i >= 0 {
			t.Errorf("the %s script still passes --password to `config credentials` (at %d); "+
				"drop the flag and let kcadm read KC_CLI_PASSWORD", phase, i)
		}
		// Matched as an assignment of the container's own variable, not as the bare name: a comment
		// or an error message mentioning KC_CLI_PASSWORD must not satisfy this.
		if !strings.Contains(script, `KC_CLI_PASSWORD="$KC_BOOTSTRAP_ADMIN_PASSWORD"`) {
			t.Errorf("the %s script does not set KC_CLI_PASSWORD from the container's "+
				"KC_BOOTSTRAP_ADMIN_PASSWORD, so `config credentials` has no password to use and "+
				"would fail with \"Console is not active, but password is required\"", phase)
		}
	}
}

// indexOfCredentialsWithPasswordFlag returns the offset of a `config credentials` command that also
// carries --password, or -1.
//
// Scoped to one command: A's scripts are newline-separated, so a --password belonging to a later
// `set-password` line must not be attributed to the login. (Operator passwords in argv are a real
// and separate exposure, recorded in docs/guard-parity.md; this guard is about the admin secret.)
func indexOfCredentialsWithPasswordFlag(script string) int {
	offset := 0
	for _, line := range strings.Split(script, "\n") {
		if strings.Contains(line, "config credentials") && strings.Contains(line, "--password") {
			return offset
		}
		offset += len(line) + 1
	}
	return -1
}

// The second exposure on this path, and the one the guard above does not cover.
//
// The admin secret was taken out of argv; each OPERATOR's password stayed in, as
// `set-password --new-password <value>`. It lands in two world-readable places: the kcadm JVM's
// argv inside the container, and — because the whole script is one argument to
// `docker exec … bash -c` — the docker client's argv on the host. `ps` shows both to any user.
//
// The audit that found it (docs/guard-parity.md) recorded it as a gap in BOTH scenarios, which is
// exactly what a parity comparison cannot find: it is not drift, it is the same hole on each side.
//
// The fix has the same shape as the admin one, because kcadm offers the same mechanism —
// `set-password --help` states that with no --new-password it reads KC_CLI_PASSWORD. What differs
// is where the value comes from: the admin secret is already inside the container
// (KC_BOOTSTRAP_ADMIN_PASSWORD, set by compose), while an operator password is not, so it is
// forwarded for that one exec with `docker exec -e NAME` — the pass-through form, which puts only
// the NAME in the docker client's argv.
//
// Verified against quay.io/keycloak/keycloak:26.0, the image this scenario pins: set-password with
// no flag and the value only in the environment succeeds; the operator then authenticates with it
// (HTTP 200) and a wrong password is refused (401); with the variable absent the command exits 1
// rather than quietly setting an empty password.

// TestKeycloakReconcile_NeverEmbedsAnOperatorPassword is the load-bearing assertion.
func TestKeycloakReconcile_NeverEmbedsAnOperatorPassword(t *testing.T) {
	for phase, call := range callsFromReconcileStep(t) {
		for _, u := range sentinelOperatorUsers {
			if strings.Contains(call.script, u.Password) {
				t.Errorf("the %s script embeds %s's password. The script is one argument to "+
					"`docker exec`, so the value is visible in `ps` on the host and inside the "+
					"container. Set KC_CLI_PASSWORD for that one command instead.", phase, u.Username)
			}
			// call.env is the child process's ENVIRONMENT, and the value belongs there — that is
			// the whole mechanism. What must never carry it is the argument vector, which is
			// asserted separately against the real builder in
			// TestDockerExecArgs_CarryNamesNeverValues.
			_ = u
		}
	}
}

// TestKeycloakReconcile_SetsOperatorPasswordsFromTheEnvironment pins the shape, which the value
// check alone cannot: a script that stopped passing --new-password and also stopped setting any
// password would satisfy the test above and leave every operator unable to sign in.
func TestKeycloakReconcile_SetsOperatorPasswordsFromTheEnvironment(t *testing.T) {
	calls := callsFromReconcileStep(t)
	run, ok := calls["run"]
	if !ok {
		t.Fatal("no run script recorded")
	}

	if strings.Contains(run.script, "--new-password") {
		t.Error("the reconcile script still passes --new-password; kcadm reads KC_CLI_PASSWORD " +
			"when the flag is absent (its own --help says so)")
	}

	// One variable per operator, and the script must reference exactly the names the exec forwards.
	// A mismatch would set an empty password: kcadm exits 1 there, so the step fails loudly rather
	// than leaving a passwordless account — but only if the two halves agree.
	setPasswordLines := 0
	for _, line := range strings.Split(run.script, "\n") {
		if !strings.Contains(line, "set-password") {
			continue
		}
		setPasswordLines++
		name, ok := operatorPasswordVarIn(line)
		if !ok {
			t.Errorf("a set-password line has no KC_CLI_PASSWORD prefix, so it would prompt or "+
				"fail:\n\t%s", line)
			continue
		}
		if !slices.ContainsFunc(run.env, func(pair string) bool {
			return strings.HasPrefix(pair, name+"=")
		}) {
			t.Errorf("the script reads $%s but the exec does not provide it; the password would "+
				"be empty and the operator could not sign in", name)
		}
	}
	if setPasswordLines != len(sentinelOperatorUsers) {
		t.Errorf("found %d set-password lines for %d declared operators; every declared operator "+
			"must get a password or it cannot sign in", setPasswordLines, len(sentinelOperatorUsers))
	}
}

// operatorPasswordVarIn returns the variable name a set-password line takes its value from, as
// `KC_CLI_PASSWORD="$NAME" …`. Matched as an assignment reading another variable, not as the bare
// name: a literal value in the prefix is the leak in a different costume.
func operatorPasswordVarIn(line string) (string, bool) {
	const prefix = `KC_CLI_PASSWORD="$`
	i := strings.Index(line, prefix)
	if i < 0 {
		return "", false
	}
	rest := line[i+len(prefix):]
	end := strings.Index(rest, `"`)
	if end <= 0 {
		return "", false
	}
	return rest[:end], true
}

// TestDockerExecArgs_CarryNamesNeverValues asserts the artefact `ps` actually prints.
//
// The rest of this file reads the script and the environment; neither would notice a secret that
// moved into a docker argument — which is the obvious way to write this and the reason the guard
// exists. `-e NAME=value` is what most examples show, and it is exactly the mistake: it relocates
// the exposure from the script to the docker client's own command line.
func TestDockerExecArgs_CarryNamesNeverValues(t *testing.T) {
	env := operatorPasswordEnv(sentinelOperatorUsers)
	args := dockerExecArgs("cb-keycloak", "echo hello", env)

	joined := strings.Join(args, " ")
	for _, u := range sentinelOperatorUsers {
		if strings.Contains(joined, u.Password) {
			t.Errorf("the docker argument vector carries %s's password:\n\t%s", u.Username, joined)
		}
	}
	for i := range sentinelOperatorUsers {
		name := operatorPasswordVar(i)
		if !slices.Contains(args, name) {
			t.Errorf("the docker argument vector does not forward %s (%v); the variable would be "+
				"unset inside the container and the password empty", name, args)
		}
		if strings.Contains(joined, name+"=") {
			t.Errorf("%s is passed as `-e NAME=value`; use the pass-through form `-e NAME`", name)
		}
	}
	// The command still has to be a command.
	if !slices.Contains(args, "cb-keycloak") || !slices.Contains(args, "bash") {
		t.Errorf("the argument vector lost the container or the shell: %v", args)
	}
}

// An exec with nothing to forward must produce the plain form, not a stray `-e`.
func TestDockerExecArgs_NoEnvIsThePlainForm(t *testing.T) {
	args := dockerExecArgs("cb-keycloak", "echo hello", nil)
	if slices.Contains(args, "-e") {
		t.Errorf("an exec with no environment emitted -e: %v", args)
	}
}

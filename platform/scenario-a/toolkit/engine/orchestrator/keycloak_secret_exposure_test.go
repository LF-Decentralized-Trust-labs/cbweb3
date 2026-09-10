// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
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

// scriptsFromReconcileStep returns every script the step handed to `docker exec … bash -c`, across
// both Check and Run.
func scriptsFromReconcileStep(t *testing.T) map[string]string {
	t.Helper()
	scripts := map[string]string{}
	newStep := func(record func(string)) *reconcileAdminUsersStep {
		return &reconcileAdminUsersStep{
			name: StepReconcileAdminUsers, entityPrefix: "cb", kcAdminPass: sentinelAdminPassword,
			realms: []KeycloakRealmPlan{{Realm: "cb-realm", Users: govAndAdmission}},
			dockerExecCmd: func(_ context.Context, _, script string) ([]byte, error) {
				record(script)
				return nil, nil
			},
		}
	}
	if _, err := newStep(func(s string) { scripts["check"] = s }).Check(context.Background()); err != nil {
		t.Fatalf("check: %v", err)
	}
	if err := newStep(func(s string) { scripts["run"] = s }).Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, phase := range []string{"check", "run"} {
		if scripts[phase] == "" {
			t.Fatalf("no script was recorded for %s; the step did not shell into Keycloak", phase)
		}
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

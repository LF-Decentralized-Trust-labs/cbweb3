// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"fmt"
	"strings"
)

// How an operator password reaches Keycloak without passing through anyone's argv.
//
// A deliberate copy of Scenario A's helpers of the same names
// (scenario-a/toolkit/engine/orchestrator/keycloak_admin_users_reconcile.go). The two toolkits
// share no library, so the rule is duplicated rather than imported; the pair of exposure guards is
// what stops them drifting. Recorded in docs/scenario-drift.md.
//
// The problem, in one line: `kcadm set-password --new-password <value>` puts the value in the
// kcadm JVM's argv inside the container, and — since the whole provisioning script is one argument
// to `docker exec … bash -c` — in the docker client's argv on the host as well. Both are visible
// to any user through `ps`.

// operatorPasswordVar is the environment variable one operator's password travels in.
//
// Indexed rather than derived from the username: a username is an email address, and @ and . are
// not valid in a shell variable name. The index is the operator's position in the same slice the
// script is generated from, so the name in the script and the value in the environment cannot
// drift — one loop over one list produces both.
func operatorPasswordVar(i int) string { return fmt.Sprintf("KC_OP_PW_%d", i) }

// operatorPasswordEnv pairs each operator's variable with its value, as "NAME=value" for os/exec.
// Only the NAMES reach docker's argv — dockerExecArgs derives them from these pairs.
func operatorPasswordEnv(users []AdminUser) []string {
	env := make([]string, 0, len(users))
	for i, u := range users {
		env = append(env, operatorPasswordVar(i)+"="+u.Password)
	}
	return env
}

// dockerExecArgs builds the docker argument vector for one exec.
//
// Split out so the argument vector can be asserted directly: it is the artefact `ps` prints, and a
// test that can only see the script would not notice a secret that moved into an argument.
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

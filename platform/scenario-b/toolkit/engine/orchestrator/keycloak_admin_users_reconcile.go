// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// Operator accounts and their realm roles are created by provision-keycloak-*, whose Check
// short-circuits as soon as KEYCLOAK_CLIENT_SECRET exists in the entity env. So on an entity that
// is ALREADY provisioned that step is skipped, and a role newly declared in spec.adminUsers never
// reaches Keycloak.
//
// That is not hypothetical: spec 042 re-gates POST /approve-kyc to ROLE_ADMISSION, a role granted
// only to an admin user declared as ADMISSION. On a fresh deploy the manifest carries it and all is
// well. On an existing stack, adding the user to the manifest and re-applying creates neither the
// realm role nor the user — so the route is re-gated to a role nobody holds and KYC approval
// becomes unreachable, with nothing in the apply report saying so.
//
// It is the same shape reconcile-noc-origins was written for, one object along: "already
// provisioned" read as "already correct". Users and their role grants are declarative and cheap, so
// they get a step that converges every run instead of riding on one that only ever runs once.
//
// Deliberately NOT declarative in the other direction: this converges towards the manifest but does
// not delete users or revoke roles it did not declare. An operator account created by hand during
// an incident is not something an apply should silently remove, and unlike the NOC client's CORS
// allowlist — where an extra origin is a real exposure — an extra operator is visible in Keycloak
// and revoking it is a deliberate act. Removing accounts belongs in its own decision, not here.

// adminUsersReadScript builds the kcadm script that reports which realm roles each declared user
// currently holds. One line per user, so a user that does not exist at all is distinguishable from
// one that exists without its roles.
func adminUsersReadScript(kc, realm string, users []AdminUser) string {
	var b strings.Builder
	b.WriteString(kcadmPreamble)
	fmt.Fprintf(&b, "%s >/dev/null 2>&1 || exit 1\n", kcadmLogin(kc))
	for _, u := range users {
		// `|| true` per user: a missing user makes get-roles exit non-zero, and that is an answer
		// (it holds nothing), not a reason to abandon the sweep.
		fmt.Fprintf(&b, "echo \"USER %[1]s\"; %[2]s get-roles -r %[3]s --uusername %[1]s --fields name 2>/dev/null || true\n",
			u.Username, kc, realm)
	}
	return b.String()
}

// parseAdminUserRoles turns the read script's output into username → set of realm roles held.
//
// It reads the `"name" : "ROLE_X"` lines kcadm emits between USER markers rather than parsing the
// JSON array, because kcadm prints one array per invocation and the markers are what tie a block to
// its user.
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
		// `"name" : "ROLE_GOVERNANCE",` → ROLE_GOVERNANCE
		parts := strings.Split(line, ":")
		if len(parts) < 2 {
			continue
		}
		value := strings.TrimSpace(parts[len(parts)-1])
		value = strings.TrimSuffix(value, ",")
		value = strings.Trim(value, "\"")
		if value != "" {
			held[current][value] = true
		}
	}
	return held
}

// adminUsersSatisfied reports whether every declared user holds every realm role its manifest role
// maps to. Missing user and missing grant are the same answer — converge — which is what makes this
// safe to run against an entity provisioned by an older version.
func adminUsersSatisfied(out string, users []AdminUser) bool {
	held := parseAdminUserRoles(out)
	for _, u := range users {
		got, ok := held[u.Username]
		if !ok {
			return false
		}
		for _, want := range realmRolesForAdminRole(u.Role) {
			if !got[want] {
				return false
			}
		}
	}
	return true
}

// adminUsersAlreadyProvisioned is the step's Check. An unreachable Keycloak answers "not
// satisfied", so the Run brings it up and converges — the same self-sufficiency
// reconcile-noc-origins relies on.
func adminUsersAlreadyProvisioned(ctx context.Context, r exec.CommandRunner, container, kc, realm string, users []AdminUser) (bool, error) {
	if len(users) == 0 {
		return true, nil
	}
	out, err := r.Run(ctx, "docker", "exec", container, "bash", "-c", adminUsersReadScript(kc, realm, users))
	if err != nil {
		return false, nil
	}
	return adminUsersSatisfied(string(out), users), nil
}

// reconcileAdminUsers creates any missing realm role, user and grant. It reuses the same builder
// the initial provisioning uses, so the two cannot drift: whatever provision-keycloak-* would have
// created on a fresh deploy is what converges here on an existing one.
func reconcileAdminUsers(ctx context.Context, r exec.CommandRunner, container, kc, realm string, users []AdminUser) error {
	if len(users) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString(kcadmPreamble)
	fmt.Fprintf(&b, "%s && ", kcadmLogin(kc))
	appendKeycloakUsers(&b, kc, realm, users)
	if _, err := r.Run(ctx, "docker", "exec", container, "bash", "-c", b.String()); err != nil {
		return fmt.Errorf("reconcile admin users: %w", err)
	}
	return nil
}

// declaredRealmRoles is the sorted set of realm roles the declared users require. Exposed for the
// report and for tests: it is the answer to "what did this apply intend to exist".
func declaredRealmRoles(users []AdminUser) []string {
	seen := map[string]bool{}
	for _, u := range users {
		for _, r := range realmRolesForAdminRole(u.Role) {
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

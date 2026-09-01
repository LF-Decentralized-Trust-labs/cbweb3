// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/scenario-b/toolkit/engine/exec"
)

// The defect these pin, found in review of the 042 admission profile: provision-keycloak-spoke
// skips as soon as KEYCLOAK_CLIENT_SECRET exists, so a role newly declared in spec.adminUsers never
// reaches an entity that is already provisioned. Spec 042 re-gates POST /approve-kyc to
// ROLE_ADMISSION, granted only to a user declared as ADMISSION — so on an upgraded stack the route
// would close on a role nobody holds and KYC approval would become unreachable, with nothing in the
// apply report saying so. Same shape as the NOC-origins defect, one object along.

func kcRolesOutput(entries map[string][]string, order ...string) string {
	var b strings.Builder
	for _, user := range order {
		b.WriteString("USER " + user + "\n")
		if roles, ok := entries[user]; ok {
			b.WriteString("[ {\n")
			for _, r := range roles {
				b.WriteString("  \"name\" : \"" + r + "\",\n")
			}
			b.WriteString("} ]\n")
		}
	}
	return b.String()
}

func TestAdminUsersSatisfied_RequiresEveryMappedRole(t *testing.T) {
	users := []AdminUser{
		{Role: "GOVERNANCE", Username: "gov@cb.test", Password: "x"},
		{Role: "ADMISSION", Username: "adm@cb.test", Password: "x"},
	}

	t.Run("all_present", func(t *testing.T) {
		out := kcRolesOutput(map[string][]string{
			"gov@cb.test": {"central_bank", "ROLE_GOVERNANCE"},
			"adm@cb.test": realmRolesForAdminRole("ADMISSION"),
		}, "gov@cb.test", "adm@cb.test")
		if !adminUsersSatisfied(out, users) {
			t.Fatal("a fully provisioned realm must not be reconciled again")
		}
	})

	// The upgrade case: the governance operator is there from the previous deploy, the admission
	// user was added to the manifest and never created.
	t.Run("declared_user_missing_entirely", func(t *testing.T) {
		out := kcRolesOutput(map[string][]string{
			"gov@cb.test": {"central_bank", "ROLE_GOVERNANCE"},
		}, "gov@cb.test", "adm@cb.test")
		if adminUsersSatisfied(out, users) {
			t.Fatal("a declared user that does not exist must trigger reconciliation — this is the upgrade that would otherwise strand approve-kyc")
		}
	})

	// The subtler one: the user exists (someone created it by hand) but holds none of its roles.
	t.Run("user_exists_without_its_roles", func(t *testing.T) {
		out := kcRolesOutput(map[string][]string{
			"gov@cb.test": {"central_bank", "ROLE_GOVERNANCE"},
			"adm@cb.test": {"offline_access"},
		}, "gov@cb.test", "adm@cb.test")
		if adminUsersSatisfied(out, users) {
			t.Fatal("a user without its mapped roles is not provisioned — a missing grant is as broken as a missing user")
		}
	})

	t.Run("partial_grant", func(t *testing.T) {
		// realmRolesForAdminRole("GOVERNANCE") returns two roles; holding one is not enough.
		out := kcRolesOutput(map[string][]string{
			"gov@cb.test": {"ROLE_GOVERNANCE"},
			"adm@cb.test": realmRolesForAdminRole("ADMISSION"),
		}, "gov@cb.test", "adm@cb.test")
		if adminUsersSatisfied(out, users) {
			t.Fatalf("holding only part of %v must not count as satisfied", realmRolesForAdminRole("GOVERNANCE"))
		}
	})
}

func TestParseAdminUserRoles_TiesRolesToTheirUser(t *testing.T) {
	// One kcadm array per user, separated by the markers — without the markers a role held by the
	// first user would read as held by all of them.
	out := kcRolesOutput(map[string][]string{
		"a@cb.test": {"ROLE_GOVERNANCE"},
		"b@cb.test": {"ROLE_ADMISSION"},
	}, "a@cb.test", "b@cb.test")

	held := parseAdminUserRoles(out)
	if !held["a@cb.test"]["ROLE_GOVERNANCE"] || held["a@cb.test"]["ROLE_ADMISSION"] {
		t.Fatalf("roles leaked across users: %v", held)
	}
	if !held["b@cb.test"]["ROLE_ADMISSION"] || held["b@cb.test"]["ROLE_GOVERNANCE"] {
		t.Fatalf("roles leaked across users: %v", held)
	}
}

func TestAdminUsersAlreadyProvisioned_UnreachableKeycloakConverges(t *testing.T) {
	// A provisioned entity with its containers down cannot answer. Reporting "satisfied" there
	// would skip the reconciliation exactly when it is needed; the Run brings Keycloak up itself.
	fake := &exec.FakeRunner{Errs: map[string]error{"docker": context.DeadlineExceeded}}
	ok, err := adminUsersAlreadyProvisioned(context.Background(), fake, "kc", "kcadm", "realm", "pw",
		[]AdminUser{{Role: "ADMISSION", Username: "adm@cb.test", Password: "x"}})
	if err != nil {
		t.Fatalf("an unreachable Keycloak is an answer, not an error: %v", err)
	}
	if ok {
		t.Fatal("must report unsatisfied so the Run converges")
	}
}

func TestAdminUsersAlreadyProvisioned_NoDeclaredUsersIsSatisfied(t *testing.T) {
	fake := &exec.FakeRunner{}
	ok, err := adminUsersAlreadyProvisioned(context.Background(), fake, "kc", "kcadm", "realm", "pw", nil)
	if err != nil || !ok {
		t.Fatalf("nothing declared means nothing to converge: ok=%v err=%v", ok, err)
	}
	if len(fake.Calls) != 0 {
		t.Fatalf("must not shell into Keycloak with nothing to check, got %d calls", len(fake.Calls))
	}
}

// The reconciliation must create exactly what the initial provisioning would have, or the two drift.
func TestReconcileAdminUsers_CreatesRoleUserAndGrant(t *testing.T) {
	fake := &exec.FakeRunner{}
	users := []AdminUser{{Role: "ADMISSION", Username: "adm@cb.test", Password: "s3cret"}}

	if err := reconcileAdminUsers(context.Background(), fake, "kc-container", "kcadm", "cbweb3b", "pw", users); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(fake.Calls) != 1 {
		t.Fatalf("expected one docker exec, got %d", len(fake.Calls))
	}
	script := strings.Join(fake.Calls[0].Args, " ")
	for _, want := range []string{
		"create roles -r cbweb3b -s name=ROLE_ADMISSION",
		"create users -r cbweb3b -s username=adm@cb.test",
		"set-password -r cbweb3b --username adm@cb.test",
		"add-roles -r cbweb3b --uusername adm@cb.test --rolename ROLE_ADMISSION",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("reconciliation script is missing %q\n%s", want, script)
		}
	}
}

func TestDeclaredRealmRoles_IsTheSetProvisioningIntends(t *testing.T) {
	got := declaredRealmRoles([]AdminUser{
		{Role: "ADMISSION", Username: "a", Password: "x"},
		{Role: "GOVERNANCE", Username: "b", Password: "x"},
		{Role: "ADMISSION", Username: "c", Password: "x"},
	})
	if len(got) == 0 {
		t.Fatal("expected the union of mapped roles")
	}
	seen := map[string]int{}
	for _, r := range got {
		seen[r]++
		if seen[r] > 1 {
			t.Fatalf("duplicate role %q in %v", r, got)
		}
	}
}

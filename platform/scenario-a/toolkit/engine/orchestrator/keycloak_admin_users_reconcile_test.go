// SPDX-License-Identifier: Apache-2.0

package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// The defect these pin, found in review of the 042 admission profile. Two things hide it in
// Scenario A: provision-keycloak seeds a realm IMPORT, which Keycloak applies only to a realm that
// does not yet exist, and that step's Check is an HTTP readiness probe, so it skips whenever
// Keycloak is up. A role newly declared in spec.adminUsers therefore never reaches an entity that is
// already provisioned.
//
// Spec 042 makes it concrete: POST /approve-kyc is re-gated to ROLE_ADMISSION, granted only to a
// user declared as ADMISSION, so on an upgraded stack the route would close on a role nobody holds
// and KYC approval would become unreachable — with nothing in the apply report saying so.

func rolesOutput(order []string, entries map[string][]string) string {
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

var govAndAdmission = []KeycloakUserPlan{
	{Username: "gov@cb.test", Password: "x", Roles: []string{"ROLE_GOVERNANCE"}},
	{Username: "adm@cb.test", Password: "x", Roles: []string{"ROLE_ADMISSION"}},
}

func TestAdminUsersSatisfied(t *testing.T) {
	order := []string{"gov@cb.test", "adm@cb.test"}

	t.Run("all_present", func(t *testing.T) {
		out := rolesOutput(order, map[string][]string{
			"gov@cb.test": {"ROLE_GOVERNANCE"}, "adm@cb.test": {"ROLE_ADMISSION"},
		})
		if !adminUsersSatisfied(out, govAndAdmission) {
			t.Fatal("a realm that already matches the manifest must not be reconciled again")
		}
	})

	// The upgrade case this step exists for.
	t.Run("declared_user_never_created", func(t *testing.T) {
		out := rolesOutput(order, map[string][]string{"gov@cb.test": {"ROLE_GOVERNANCE"}})
		if adminUsersSatisfied(out, govAndAdmission) {
			t.Fatal("a declared user that does not exist must trigger reconciliation — otherwise approve-kyc is gated on a role nobody holds")
		}
	})

	t.Run("user_exists_without_its_role", func(t *testing.T) {
		out := rolesOutput(order, map[string][]string{
			"gov@cb.test": {"ROLE_GOVERNANCE"}, "adm@cb.test": {"offline_access"},
		})
		if adminUsersSatisfied(out, govAndAdmission) {
			t.Fatal("a missing grant is as broken as a missing user")
		}
	})
}

func TestParseAdminUserRoles_TiesRolesToTheirUser(t *testing.T) {
	out := rolesOutput([]string{"a@cb.test", "b@cb.test"}, map[string][]string{
		"a@cb.test": {"ROLE_GOVERNANCE"}, "b@cb.test": {"ROLE_ADMISSION"},
	})
	held := parseAdminUserRoles(out)
	if !held["a@cb.test"]["ROLE_GOVERNANCE"] || held["a@cb.test"]["ROLE_ADMISSION"] {
		t.Fatalf("roles leaked across users: %v", held)
	}
	if !held["b@cb.test"]["ROLE_ADMISSION"] || held["b@cb.test"]["ROLE_GOVERNANCE"] {
		t.Fatalf("roles leaked across users: %v", held)
	}
}

func TestReconcileScript_CreatesRoleUserPasswordAndGrant(t *testing.T) {
	script := adminUsersReconcileScript(keycloakAdminCLI, "cb-realm",
		[]KeycloakUserPlan{{Username: "adm@cb.test", Password: "s3cret", Roles: []string{"ROLE_ADMISSION"}}})

	for _, want := range []string{
		"create roles -r cb-realm -s name=ROLE_ADMISSION",
		"create users -r cb-realm -s username=adm@cb.test",
		"emailVerified=true",
		"set-password -r cb-realm --username adm@cb.test --new-password s3cret",
		"add-roles -r cb-realm --uusername adm@cb.test --rolename ROLE_ADMISSION",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q\n%s", want, script)
		}
	}
	// Every command tolerates "already exists", or a second apply on a correct realm would fail.
	for _, line := range strings.Split(strings.TrimSpace(script), "\n") {
		if strings.Contains(line, "config credentials") || line == "" {
			continue
		}
		if !strings.HasSuffix(line, "|| true") {
			t.Fatalf("command is not idempotent-tolerant: %q", line)
		}
	}
}

// A provisioned entity whose containers are down cannot answer. Reporting "satisfied" there would
// skip reconciliation exactly when it is needed.
func TestCheck_UnreachableKeycloakConverges(t *testing.T) {
	s := &reconcileAdminUsersStep{
		name: StepReconcileAdminUsers, entityPrefix: "cb", kcAdminPass: "pw",
		realms: []KeycloakRealmPlan{{Realm: "cb-realm", Users: govAndAdmission}},
		dockerExecCmd: func(context.Context, string, string) ([]byte, error) {
			return nil, errors.New("container not running")
		},
	}
	ok, err := s.Check(context.Background())
	if err != nil {
		t.Fatalf("an unreachable Keycloak is an answer, not an error: %v", err)
	}
	if ok {
		t.Fatal("must report unsatisfied so the Run converges")
	}
}

func TestCheck_NoDeclaredUsersNeverShellsIn(t *testing.T) {
	calls := 0
	s := &reconcileAdminUsersStep{
		name: StepReconcileAdminUsers, entityPrefix: "cb", kcAdminPass: "pw",
		realms: []KeycloakRealmPlan{{Realm: "cb-realm"}},
		dockerExecCmd: func(context.Context, string, string) ([]byte, error) {
			calls++
			return nil, nil
		},
	}
	ok, err := s.Check(context.Background())
	if err != nil || !ok {
		t.Fatalf("nothing declared means nothing to converge: ok=%v err=%v", ok, err)
	}
	if calls != 0 {
		t.Fatalf("must not shell into Keycloak with nothing to check, got %d calls", calls)
	}
}

func TestRun_ConvergesEveryRealmThatDeclaresUsers(t *testing.T) {
	var realmsTouched []string
	s := &reconcileAdminUsersStep{
		name: StepReconcileAdminUsers, entityPrefix: "cb", kcAdminPass: "pw",
		realms: []KeycloakRealmPlan{
			{Realm: "cb-realm", Users: govAndAdmission},
			{Realm: "noc", Users: []KeycloakUserPlan{{Username: "noc@cb.test", Password: "x", Roles: []string{"ROLE_NOC_ADMIN"}}}},
			{Realm: "empty"},
		},
		dockerExecCmd: func(_ context.Context, _, script string) ([]byte, error) {
			for _, r := range []string{"cb-realm", "noc", "empty"} {
				if strings.Contains(script, "-r "+r+" ") {
					realmsTouched = append(realmsTouched, r)
					break
				}
			}
			return nil, nil
		},
	}
	if err := s.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(realmsTouched) != 2 {
		t.Fatalf("expected the two realms that declare users, got %v", realmsTouched)
	}
	for _, r := range realmsTouched {
		if r == "empty" {
			t.Fatal("a realm declaring no users must not be touched")
		}
	}
}

// The step must be in the canonical order, or the order test fails and the step silently never runs
// in a planned apply.
func TestReconcileAdminUsersIsInTheCanonicalFoundOrder(t *testing.T) {
	found := false
	for i, name := range CanonicalStepOrder {
		if name == StepReconcileAdminUsers {
			found = true
			if i == 0 || CanonicalStepOrder[i-1] != StepProvisionKeycloak {
				t.Fatalf("reconcile-admin-users must follow provision-keycloak, got predecessor %q", CanonicalStepOrder[i-1])
			}
		}
	}
	if !found {
		t.Fatal("reconcile-admin-users is missing from the canonical found order")
	}
}

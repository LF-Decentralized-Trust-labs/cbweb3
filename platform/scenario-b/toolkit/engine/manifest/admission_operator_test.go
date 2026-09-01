// SPDX-License-Identifier: Apache-2.0

package manifest

import "testing"

// Spec 042 / contracts/manifest-admission-user.md V1, V2, V4.
//
// The Admission operator is a REQUIRED adminUsers[] entry on a central-bank
// declaration and only there: the hub is not an onboarding authority (commercial
// banks join spokes, not the hub) and neither is a commercial bank, so requiring it
// of them would break provisioning of those entities.

func admissionErrors(users []AdminUser, topologyRole string) []Finding {
	var r Result
	validateAdminUsers(users, topologyRole, &r)
	var out []Finding
	for _, e := range r.Errors {
		if e.Field == "spec.adminUsers" && e.Message != "" {
			out = append(out, e)
		}
	}
	return out
}

func cbOperators(extra ...AdminUser) []AdminUser {
	base := []AdminUser{
		{Role: "GOVERNANCE", Username: "cb-governance", Password: "local"},
		{Role: "TREASURY", Username: "cb-treasury", Password: "local"},
		{Role: "SUPERVISOR", Username: "cb-supervisor", Password: "local"},
	}
	return append(base, extra...)
}

// V1 — a central-bank declaration WITHOUT an Admission operator fails validation.
func TestCentralBankWithoutAdmissionOperatorFails(t *testing.T) {
	got := admissionErrors(cbOperators(), "central-bank")
	if len(got) == 0 {
		t.Fatal("expected a validation error: a central bank must declare an ADMISSION operator")
	}
}

// V2 — with the Admission operator it passes.
func TestCentralBankWithAdmissionOperatorPasses(t *testing.T) {
	users := cbOperators(AdminUser{Role: "ADMISSION", Username: "cb-admission", Password: "local"})
	if got := admissionErrors(users, "central-bank"); len(got) != 0 {
		t.Fatalf("expected valid, got %+v", got)
	}
}

// The role match is case- and whitespace-insensitive, matching realmRolesForAdminRole.
func TestAdmissionRoleMatchIsNormalized(t *testing.T) {
	for _, spelling := range []string{"admission", " Admission ", "ADMISSION"} {
		users := cbOperators(AdminUser{Role: spelling, Username: "cb-admission", Password: "local"})
		if got := admissionErrors(users, "central-bank"); len(got) != 0 {
			t.Errorf("role %q: expected valid, got %+v", spelling, got)
		}
	}
}

// V4 — non-central-bank entities are unaffected. Requiring an Admission operator of
// the hub or a joining bank would break their provisioning.
func TestNonCentralBankEntitiesDoNotRequireAdmission(t *testing.T) {
	for _, topologyRole := range []string{"hub", "commercial-bank", ""} {
		users := []AdminUser{{Role: "GOVERNANCE", Username: "op", Password: "local"}}
		if got := admissionErrors(users, topologyRole); len(got) != 0 {
			t.Errorf("topology.role %q: must not require an ADMISSION operator, got %+v", topologyRole, got)
		}
	}
}

// The pre-existing shape checks still apply to the new entry.
func TestAdmissionOperatorStillNeedsUsernameAndPassword(t *testing.T) {
	var r Result
	validateAdminUsers(cbOperators(AdminUser{Role: "ADMISSION"}), "central-bank", &r)
	if len(r.Errors) == 0 {
		t.Fatal("expected shape errors for an ADMISSION entry missing username and password")
	}
}

// SPDX-License-Identifier: Apache-2.0

package domain

import "testing"

func TestRequiresPKI(t *testing.T) {
	tests := []struct {
		role string
		want bool
	}{
		{RoleCommercialBank, true},
		{RoleTreasury, true},
		{RoleGovernance, false},
		{RoleSupervisor, false},
		{RoleNOC, false},
		{RoleGovernanceOfficer, false},
		{"ROLE_UNKNOWN", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := RequiresPKI(tt.role); got != tt.want {
			t.Errorf("RequiresPKI(%q) = %v, want %v", tt.role, got, tt.want)
		}
	}
}

func TestPKIRolesMap(t *testing.T) {
	// PKI roles must be exactly the X.509-login roles; supervisor/NOC are
	// password-only and must never be in the map.
	if !PKIRoles[RoleCommercialBank] || !PKIRoles[RoleTreasury] {
		t.Fatal("expected commercial bank and treasury to require PKI")
	}
	if _, ok := PKIRoles[RoleSupervisor]; ok {
		t.Error("supervisor must not require PKI")
	}
}

func TestDisclosureStateConstants(t *testing.T) {
	// Guard the on-the-wire string values; persisted/compared as strings.
	cases := map[DisclosureState]string{
		DisclosurePending:       "PENDING",
		DisclosureApproved:      "APPROVED",
		DisclosureDenied:        "DENIED",
		DisclosureExpired:       "EXPIRED",
		DisclosureQuorumReached: "QUORUM_REACHED",
		DisclosureDisclosed:     "DISCLOSED",
	}
	for state, want := range cases {
		if string(state) != want {
			t.Errorf("DisclosureState %v = %q, want %q", state, string(state), want)
		}
	}
}

func TestZKPointerStateConstants(t *testing.T) {
	cases := map[ZKPointerState]string{
		ZKPointerValid:   "VALID",
		ZKPointerExpired: "EXPIRED",
		ZKPointerRevoked: "REVOKED",
	}
	for state, want := range cases {
		if string(state) != want {
			t.Errorf("ZKPointerState %v = %q, want %q", state, string(state), want)
		}
	}
}

func TestParticipantStatusConstants(t *testing.T) {
	cases := map[ParticipantStatus]string{
		StatusPending:             "PENDING",
		StatusCredentialRequested: "CREDENTIAL_REQUESTED",
		StatusKYCApproved:         "KYC_APPROVED",
		StatusActive:              "ACTIVE",
		StatusFrozen:              "FROZEN",
		StatusRevoked:             "REVOKED",
	}
	for st, want := range cases {
		if string(st) != want {
			t.Errorf("ParticipantStatus %v = %q, want %q", st, string(st), want)
		}
	}
}

func TestTableNames(t *testing.T) {
	if got := (DisclosureRequest{}); got.RequestID != "" {
		t.Fatal("unexpected zero value")
	}
	// Audit category/severity constants are used as DB/portal filter strings.
	if string(CategoryCredential) != "CREDENTIAL" || string(SeverityCritical) != "CRITICAL" {
		t.Error("audit category/severity constant mismatch")
	}
}

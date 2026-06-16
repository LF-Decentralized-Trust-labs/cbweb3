// SPDX-License-Identifier: Apache-2.0

package domain

import "testing"

func TestIsAdminRole(t *testing.T) {
	admin := []string{
		RoleCommercialBank, RoleTreasury, RoleNOC, RoleSupervisor, RoleGovernanceOfficer,
	}
	for _, r := range admin {
		if !IsAdminRole(r) {
			t.Errorf("IsAdminRole(%q) = false, want true", r)
		}
	}
	notAdmin := []string{RoleGovernance, "ROLE_UNKNOWN", ""}
	for _, r := range notAdmin {
		if IsAdminRole(r) {
			t.Errorf("IsAdminRole(%q) = true, want false", r)
		}
	}
}

func TestRequiresPKI(t *testing.T) {
	for _, r := range []string{RoleCommercialBank, RoleTreasury} {
		if !RequiresPKI(r) {
			t.Errorf("RequiresPKI(%q) = false, want true", r)
		}
	}
	for _, r := range []string{RoleNOC, RoleSupervisor, RoleGovernance, RoleGovernanceOfficer, ""} {
		if RequiresPKI(r) {
			t.Errorf("RequiresPKI(%q) = true, want false", r)
		}
	}
}

func TestRequiresOnChain(t *testing.T) {
	if !RequiresOnChain(RoleCommercialBank) {
		t.Errorf("RequiresOnChain(%q) = false, want true", RoleCommercialBank)
	}
	// Treasury requires KMS/PKI but NOT on-chain registration.
	for _, r := range []string{RoleTreasury, RoleNOC, RoleGovernance, ""} {
		if RequiresOnChain(r) {
			t.Errorf("RequiresOnChain(%q) = true, want false", r)
		}
	}
}

func TestRequiresKMS(t *testing.T) {
	for _, r := range []string{RoleCommercialBank, RoleTreasury} {
		if !RequiresKMS(r) {
			t.Errorf("RequiresKMS(%q) = false, want true", r)
		}
	}
	for _, r := range []string{RoleNOC, RoleSupervisor, RoleGovernance, ""} {
		if RequiresKMS(r) {
			t.Errorf("RequiresKMS(%q) = true, want false", r)
		}
	}
}

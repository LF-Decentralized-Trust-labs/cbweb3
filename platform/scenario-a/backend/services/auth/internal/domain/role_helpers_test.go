// SPDX-License-Identifier: Apache-2.0

package domain

import "testing"

func TestIsAdminRole(t *testing.T) {
	t.Parallel()
	admin := []string{RoleCommercialBank, RoleTreasury, RoleNOC, RoleSupervisor, RoleGovernanceOfficer}
	for _, r := range admin {
		if !IsAdminRole(r) {
			t.Errorf("%s should be an admin role", r)
		}
	}
	for _, r := range []string{RoleGovernance, "ROLE_BOGUS", ""} {
		if IsAdminRole(r) {
			t.Errorf("%s should not be an admin role", r)
		}
	}
}

func TestRequiresPKI(t *testing.T) {
	t.Parallel()
	for _, r := range []string{RoleCommercialBank, RoleTreasury} {
		if !RequiresPKI(r) {
			t.Errorf("%s should require PKI", r)
		}
	}
	for _, r := range []string{RoleNOC, RoleSupervisor, RoleGovernance, ""} {
		if RequiresPKI(r) {
			t.Errorf("%s should not require PKI", r)
		}
	}
}

func TestRequiresOnChain(t *testing.T) {
	t.Parallel()
	if !RequiresOnChain(RoleCommercialBank) {
		t.Error("commercial bank should require on-chain registration")
	}
	for _, r := range []string{RoleTreasury, RoleNOC, RoleGovernance, ""} {
		if RequiresOnChain(r) {
			t.Errorf("%s should not require on-chain", r)
		}
	}
}

func TestRequiresKMS(t *testing.T) {
	t.Parallel()
	for _, r := range []string{RoleCommercialBank, RoleTreasury} {
		if !RequiresKMS(r) {
			t.Errorf("%s should require KMS", r)
		}
	}
	for _, r := range []string{RoleNOC, RoleSupervisor, ""} {
		if RequiresKMS(r) {
			t.Errorf("%s should not require KMS", r)
		}
	}
}

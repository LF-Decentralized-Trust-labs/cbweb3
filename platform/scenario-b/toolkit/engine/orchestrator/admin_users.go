// SPDX-License-Identifier: Apache-2.0

package orchestrator

import "strings"

// AdminUser is a per-role operator account seeded into an entity's Keycloak realm
// (mirrors the manifest's spec.adminUsers). Username is used as the Keycloak
// username AND email; Password is a local-only bootstrap value, not a secret key.
type AdminUser struct {
	Role     string
	Username string
	Password string
}

// realmRolesForAdminRole maps a manifest admin-user role to the Keycloak realm
// roles the api-gateway checks. The manifest role names follow scenario-a's
// convention (GOVERNANCE/TREASURY/SUPERVISOR/NOC for a CB, BANK for a commercial
// bank); the app-level roles (central_bank/commercial_bank + ROLE_*) are attached
// here so a governance or treasury operator can drive the sovereign AMM locally.
func realmRolesForAdminRole(role string) []string {
	switch strings.ToUpper(strings.TrimSpace(role)) {
	case "GOVERNANCE":
		return []string{"central_bank", "ROLE_GOVERNANCE"}
	case "TREASURY":
		return []string{"central_bank", "ROLE_TREASURY"}
	case "SUPERVISOR":
		return []string{"ROLE_SUPERVISOR"}
	case "ADMISSION":
		// Onboarding operator (spec 042). central_bank is attached so the operator can
		// reach the CB-scoped v2 surfaces the portal shares; ROLE_ADMISSION is the claim
		// the api-gateway's onboarding guards check. This case is REQUIRED: without it
		// the default branch below returns the bare "ADMISSION" string, the realm role
		// and JWT claim land as "ADMISSION", and every RequireRole("ROLE_ADMISSION")
		// guard fails silently. The profile is password-only — no PKI, no on-chain role.
		return []string{"central_bank", "ROLE_ADMISSION"}
	case "NOC", "NOC_ADMIN":
		return []string{"ROLE_NOC_ADMIN"}
	case "BANK", "COMMERCIAL_BANK":
		return []string{"commercial_bank", "ROLE_COMMERCIAL_BANK"}
	default:
		return []string{role}
	}
}

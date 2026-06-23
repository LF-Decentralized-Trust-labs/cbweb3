// SPDX-License-Identifier: Apache-2.0

package registry

import "strings"

// RoleToSolidityEnum converts a platform role string to the uint8 value of the
// IdentityRegistryLibrary.ParticipantRole enum. It accepts both the canonical
// "ROLE_"-prefixed form ("ROLE_COMMERCIAL_BANK") and the bare form used by the
// onboarding flow ("commercial_bank") — case-insensitive — so on-chain
// registration sets a non-NONE role (required for IdentityRegistry.canTransact)
// regardless of which form the caller supplies.
func RoleToSolidityEnum(role string) uint8 {
	r := strings.ToUpper(strings.TrimSpace(role))
	r = strings.TrimPrefix(r, "ROLE_")
	switch r {
	case "COMMERCIAL_BANK":
		return RoleCommercialBank
	case "TREASURY":
		return RoleTreasury
	case "GOVERNANCE":
		return RoleGovernance
	case "CENTRAL_BANK":
		return RoleCentralBank
	default:
		return RoleNone
	}
}

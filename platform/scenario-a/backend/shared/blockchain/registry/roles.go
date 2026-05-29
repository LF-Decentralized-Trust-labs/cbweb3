package registry

import "strings"

// RoleToSolidityEnum converts a platform role string (e.g. "ROLE_COMMERCIAL_BANK")
// to the uint8 value of the IdentityRegistryLibrary.ParticipantRole enum.
func RoleToSolidityEnum(role string) uint8 {
	switch strings.ToUpper(strings.TrimSpace(role)) {
	case "ROLE_COMMERCIAL_BANK":
		return RoleCommercialBank
	case "ROLE_TREASURY":
		return RoleTreasury
	case "ROLE_GOVERNANCE":
		return RoleGovernance
	case "ROLE_CENTRAL_BANK":
		return RoleCentralBank
	default:
		return RoleNone
	}
}

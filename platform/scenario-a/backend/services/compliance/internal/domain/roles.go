package domain

// Participant roles — must match Keycloak realm roles.
const (
	RoleGovernance        = "ROLE_GOVERNANCE"        // Banco Central (CA Root authority)
	RoleCommercialBank    = "ROLE_COMMERCIAL_BANK"   // Requires X.509 PKI login
	RoleTreasury          = "ROLE_TREASURY"          // Requires X.509 PKI login
	RoleSupervisor        = "ROLE_SUPERVISOR"        // Password-only login
	RoleNOC               = "ROLE_NOC"               // Password-only login
	RoleGovernanceOfficer = "ROLE_GOVERNANCE_OFFICER" // Password-only login
)

// PKIRoles lists the roles that require an X.509 certificate for authentication.
var PKIRoles = map[string]bool{
	RoleCommercialBank: true,
	RoleTreasury:       true,
}

// RequiresPKI returns true if the given role requires PKI-based authentication.
func RequiresPKI(role string) bool {
	return PKIRoles[role]
}

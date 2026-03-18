package domain

// Role constants — canonical participant roles across the platform.
// Legacy constants (without ROLE_ prefix) kept for backward-compat with
// existing Keycloak realm roles and token claims.
const (
	RoleCentralBank    = "CENTRAL_BANK"
	RoleCommercialBank = "COMMERCIAL_BANK"
	RoleMLP            = "MLP"
)

// Prefixed role constants used for new RBAC-aware registration flow.
// These map to Keycloak realm roles created with the ROLE_ prefix.
const (
	RoleCommercialBankPrefixed = "ROLE_COMMERCIAL_BANK"
	RoleNOC                    = "ROLE_NOC"
	RoleSupervisor             = "ROLE_SUPERVISOR"
	RoleTreasury               = "ROLE_TREASURY"
	RoleGovernance             = "ROLE_GOVERNANCE"
)

// adminRoles is the set of roles that are created/managed by Central Bank
// administrators via POST /compliance/register.
var adminRoles = map[string]bool{
	RoleCommercialBankPrefixed: true,
	RoleNOC:                    true,
	RoleSupervisor:             true,
	RoleTreasury:               true,
	RoleGovernance:             true,
}

// IsAdminRole reports whether role is an administrative role that may be
// assigned by the Central Bank during participant onboarding.
func IsAdminRole(role string) bool {
	return adminRoles[role]
}

// onChainRoles is the subset of admin roles that require on-chain registration
// in the ParticipantRegistry smart contract.
var onChainRoles = map[string]bool{
	RoleCommercialBankPrefixed: true,
	RoleNOC:                    true,
}

// RequiresOnChain reports whether role demands on-chain registration via
// ParticipantRegistry.registerMember (signed with CB_PRIVATE_KEY).
func RequiresOnChain(role string) bool {
	return onChainRoles[role]
}

// kmsRoles is the subset of admin roles for which the platform generates and
// holds an EVM key pair in the KMS on behalf of the participant.
var kmsRoles = map[string]bool{
	RoleCommercialBankPrefixed: true,
}

// RequiresKMS reports whether role demands automatic KMS key generation for
// the participant at onboarding time.
func RequiresKMS(role string) bool {
	return kmsRoles[role]
}

// KYCStatus is the canonical KYC lifecycle enum (D7 §7.1).
type KYCStatus string

const (
	KYCStatusPending  KYCStatus = "PENDING"
	KYCStatusApproved KYCStatus = "APPROVED"
	KYCStatusFrozen   KYCStatus = "FROZEN"
	KYCStatusRevoked  KYCStatus = "REVOKED"
)

// TokenClaims is the normalized token claims payload (D7 §7.4).
type TokenClaims struct {
	Subject      string   `json:"subject"`
	Issuer       string   `json:"issuer"`
	Roles        []string `json:"roles"`
	DID          string   `json:"did,omitempty"`
	Wallet       string   `json:"wallet,omitempty"`
	Country      string   `json:"country,omitempty"`
	BankID       string   `json:"bank_id,omitempty"`
	PrivacyGroup string   `json:"privacy_group,omitempty"`
}

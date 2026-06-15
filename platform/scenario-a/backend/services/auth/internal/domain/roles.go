// SPDX-License-Identifier: Apache-2.0

package domain

// PKI role constants — canonical participant roles across the platform.
// All roles use the ROLE_ prefix to match Keycloak realm role names.
const (
	RoleGovernance        = "ROLE_GOVERNANCE"         // Banco Central (CA Root authority)
	RoleCommercialBank    = "ROLE_COMMERCIAL_BANK"    // Requires X.509 PKI login
	RoleTreasury          = "ROLE_TREASURY"           // Requires X.509 PKI login
	RoleSupervisor        = "ROLE_SUPERVISOR"         // Password-only login
	RoleNOC               = "ROLE_NOC"                // Password-only login
	RoleGovernanceOfficer = "ROLE_GOVERNANCE_OFFICER" // Password-only login
)

// adminRoles is the set of roles created/managed by the Central Bank.
var adminRoles = map[string]bool{
	RoleCommercialBank:    true,
	RoleTreasury:          true,
	RoleNOC:               true,
	RoleSupervisor:        true,
	RoleGovernanceOfficer: true,
}

// IsAdminRole reports whether role may be assigned by the Central Bank during onboarding.
func IsAdminRole(role string) bool {
	return adminRoles[role]
}

// pkiRoles is the subset of roles that require X.509 PKI certificate authentication.
var pkiRoles = map[string]bool{
	RoleCommercialBank: true,
	RoleTreasury:       true,
}

// RequiresPKI returns true if the given role requires PKI-based 2FA login.
func RequiresPKI(role string) bool {
	return pkiRoles[role]
}

// onChainRoles is the subset of roles that require on-chain registration
// in the IdentityRegistry contract. Only roles that exist in the Solidity
// ParticipantRole enum are included.
var onChainRoles = map[string]bool{
	RoleCommercialBank: true,
}

// RequiresOnChain reports whether role demands on-chain registration.
func RequiresOnChain(role string) bool {
	return onChainRoles[role]
}

// kmsRoles is the subset of roles for which the platform generates a KMS key pair.
var kmsRoles = map[string]bool{
	RoleCommercialBank: true,
	RoleTreasury:       true,
}

// RequiresKMS reports whether role demands automatic KMS key generation.
func RequiresKMS(role string) bool {
	return kmsRoles[role]
}

// ParticipantStatus is the lifecycle state of a registered participant.
type ParticipantStatus string

const (
	ParticipantStatusPending             ParticipantStatus = "PENDING"
	ParticipantStatusCredentialRequested ParticipantStatus = "CREDENTIAL_REQUESTED" //#nosec G101 -- not a secret; participant lifecycle status enum value
	ParticipantStatusKYCApproved         ParticipantStatus = "KYC_APPROVED"
	ParticipantStatusActive              ParticipantStatus = "ACTIVE"
	ParticipantStatusFrozen              ParticipantStatus = "FROZEN"
	ParticipantStatusRevoked             ParticipantStatus = "REVOKED"
)

// validTransitions defines the allowed state machine for participant lifecycle.
var validTransitions = map[ParticipantStatus][]ParticipantStatus{
	ParticipantStatusPending:             {ParticipantStatusCredentialRequested, ParticipantStatusActive},
	ParticipantStatusCredentialRequested: {ParticipantStatusKYCApproved, ParticipantStatusRevoked},
	ParticipantStatusKYCApproved:         {ParticipantStatusActive, ParticipantStatusRevoked},
	ParticipantStatusActive:              {ParticipantStatusFrozen, ParticipantStatusRevoked},
	ParticipantStatusFrozen:              {ParticipantStatusActive, ParticipantStatusRevoked},
}

// IsValidTransition reports whether moving from current to next is allowed.
func IsValidTransition(current, next ParticipantStatus) bool {
	for _, allowed := range validTransitions[current] {
		if allowed == next {
			return true
		}
	}
	return false
}

// TokenClaims is the normalized token claims payload extracted from a Keycloak JWT.
type TokenClaims struct {
	Subject      string   `json:"subject"`
	Issuer       string   `json:"issuer"`
	Roles        []string `json:"roles"`
	Wallet       string   `json:"wallet,omitempty"`
	Country      string   `json:"country,omitempty"`
	BankID       string   `json:"bank_id,omitempty"`
	PrivacyGroup string   `json:"privacy_group,omitempty"`
}

// SPDX-License-Identifier: Apache-2.0

// This file defines core auth and identity domain models and related errors.
package domain

import "errors"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrWalletAlreadyBound = errors.New("wallet already bound")
	ErrUserAlreadyBound   = errors.New("user already bound to another wallet")
	ErrInvalidToken       = errors.New("invalid token")
	ErrInvalidSignature   = errors.New("invalid signature")
	ErrRejectedByKYC      = errors.New("kyc status does not allow wallet binding")
)

// AuthToken represents the token response returned by auth providers.
type AuthToken struct {
	AccessToken      string
	RefreshToken     string
	ExpiresIn        int
	RefreshExpiresIn int
	TokenType        string
}

// TokenClaims represents enriched token claims used by handlers/middleware (D7 §7.4).
type TokenClaims struct {
	Subject      string
	Issuer       string
	Scope        string
	Roles        []string
	Wallet       string
	Country      string
	BankID       string
	PrivacyGroup string
}

// WalletBinding represents a user-wallet association.
type WalletBinding struct {
	UserID        string
	WalletAddress string
}

// KYCStatus describes a subject compliance lifecycle status.
type KYCStatus string

const (
	KYCActive   KYCStatus = "ACTIVE"   // canonical active status in compliance-orchestrator
	KYCApproved KYCStatus = "APPROVED" // legacy alias — prefer KYCActive for new code
	KYCPending  KYCStatus = "PENDING"
	KYCFrozen   KYCStatus = "FROZEN"  // account frozen by Central Bank (REQ-COM-003)
	KYCRevoked  KYCStatus = "REVOKED" // credential revoked / permanent ban
	// KYCRejected kept for backward-compat with existing tests and code.
	KYCRejected KYCStatus = "REJECTED"
)

// Canonical participant role constants (mirrors identity service roles).
const (
	RoleGovernance        = "ROLE_GOVERNANCE"
	RoleCommercialBank    = "ROLE_COMMERCIAL_BANK"
	RoleTreasury          = "ROLE_TREASURY"
	RoleSupervisor        = "ROLE_SUPERVISOR"
	RoleNOC               = "ROLE_NOC"
	RoleGovernanceOfficer = "ROLE_GOVERNANCE_OFFICER"

	// Legacy constants kept for backward-compat with existing tests.
	RoleCentralBank = "CENTRAL_BANK"
	RoleMLP         = "MLP"

	// Scenario B Keycloak realm roles (FR-030 / FR-056 / M1 / M2).
	RoleCentralBankScenarioB    = "central_bank"
	RoleCommercialBankScenarioB = "commercial_bank"
	RoleMLPScenarioB            = "mlp"
)

// adminRoles is the set of roles that the Central Bank may assign during onboarding.
var adminRoles = map[string]bool{
	RoleCommercialBank:    true,
	RoleTreasury:          true,
	RoleNOC:               true,
	RoleSupervisor:        true,
	RoleGovernanceOfficer: true,
}

// IsAdminRole reports whether the given role may be assigned by the Central Bank.
func IsAdminRole(role string) bool {
	return adminRoles[role]
}

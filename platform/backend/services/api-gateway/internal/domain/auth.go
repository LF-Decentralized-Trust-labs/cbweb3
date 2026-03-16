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
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	TokenType    string
}

// TokenClaims represents enriched token claims used by handlers/middleware (D7 §7.4).
type TokenClaims struct {
	Subject      string
	Issuer       string
	Scope        string
	Roles        []string
	DID          string
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
	KYCApproved KYCStatus = "APPROVED"
	KYCPending  KYCStatus = "PENDING"
	KYCFrozen   KYCStatus = "FROZEN"  // account frozen by Central Bank (REQ-COM-003)
	KYCRevoked  KYCStatus = "REVOKED" // credential revoked / permanent ban
	// KYCRejected kept for backward-compat with existing tests and code.
	KYCRejected KYCStatus = "REJECTED"
)

// Canonical participant role constants (mirrors identityprovider package).
const (
	RoleCentralBank    = "CENTRAL_BANK"
	RoleCommercialBank = "COMMERCIAL_BANK"
	RoleMLP            = "MLP"
)

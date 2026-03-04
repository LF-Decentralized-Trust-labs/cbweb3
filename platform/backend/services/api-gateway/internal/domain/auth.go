// This file defines core auth and identity domain models and related errors.
package domain

import "errors"

var (
	// Domain-level errors used across auth and wallet binding flows.
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrWalletAlreadyBound = errors.New("wallet already bound")
	ErrUserAlreadyBound   = errors.New("user already bound to another wallet")
	ErrInvalidToken       = errors.New("invalid token")
	ErrInvalidSignature   = errors.New("invalid signature")
	ErrRejectedByKYC      = errors.New("kyc status rejected")
)

// AuthToken represents the token response returned by auth providers.
type AuthToken struct {
	AccessToken string
	ExpiresIn   int
	TokenType   string
}

// TokenClaims represents normalized token claims used by handlers/middleware.
type TokenClaims struct {
	Subject string
	Issuer  string
	Scope   string
	Roles   []string
}

// WalletBinding represents a user-wallet association.
type WalletBinding struct {
	UserID        string
	WalletAddress string
}

// KYCStatus describes a subject compliance status.
type KYCStatus string

const (
	// Known KYC statuses for compliance checks.
	KYCApproved KYCStatus = "APPROVED"
	KYCPending  KYCStatus = "PENDING"
	KYCRejected KYCStatus = "REJECTED"
)


package identityprovider

import "context"

// Role constants — canonical participant roles across the platform.
const (
	RoleCentralBank    = "CENTRAL_BANK"
	RoleCommercialBank = "COMMERCIAL_BANK"
	RoleMLP            = "MLP"
)

// KYCStatus is the canonical KYC lifecycle enum (D7 §7.1, reconciled).
type KYCStatus string

const (
	KYCStatusPending  KYCStatus = "PENDING"  // awaiting Central Bank approval
	KYCStatusApproved KYCStatus = "APPROVED" // KYC approved, operations allowed
	KYCStatusFrozen   KYCStatus = "FROZEN"   // frozen by Central Bank order (REQ-COM-003)
	KYCStatusRevoked  KYCStatus = "REVOKED"  // credential revoked / permanent ban
)

// LoginRequest is the normalized login payload for identity providers.
type LoginRequest struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

// TokenResponse is the normalized auth token payload returned by providers.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// WalletResponse is the normalized wallet creation response.
type WalletResponse struct {
	DID       string `json:"did"`
	Address   string `json:"address"`
	CreatedAt string `json:"created_at"`
}

// WalletBinding represents a user-wallet association.
type WalletBinding struct {
	UserID        string `json:"user_id"`
	WalletAddress string `json:"wallet_address"`
}

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

// SignRequest carries the digest to be signed and the signing identity.
type SignRequest struct {
	UserID    string `json:"user_id"`
	DigestHex string `json:"digest_hex"` // 32-byte hex (keccak256 of tx)
}

// SignResponse carries the resulting signature and the signer address.
type SignResponse struct {
	Signature     string `json:"signature"`      // hex-encoded secp256k1 signature
	Address       string `json:"address"`        // EVM address of signing key
	SignerProvider string `json:"signer_provider"` // e.g. "local", "dwallet_api"
}

// KYCCredentialRequest is the input for issuing a Verifiable Credential.
type KYCCredentialRequest struct {
	Subject       string `json:"subject"`        // userID of the participant being credentialed
	IssuerSubject string `json:"issuer_subject"` // userID of the Central Bank issuing the credential
	InstitutionName string `json:"institution_name"`
	CountryCode   string `json:"country_code"`
	BankCode      string `json:"bank_code"`
}

// KYCCredentialResponse carries the issued VC and on-chain ZK pointer.
type KYCCredentialResponse struct {
	VCJWT      string `json:"vc_jwt"`      // Verifiable Credential JWT
	ZKPPointer string `json:"zkp_pointer"` // SHA-256 of VC payload — on-chain pointer
	IssuedAt   string `json:"issued_at"`
}

// Provider defines the full identity + KYC capabilities for the platform.
// All identity operations MUST go through this interface — no service
// accesses identity logic directly. In production, DWalletAPIProvider
// delegates to LNET (D-Wallet API + SSI-VC-API + Claims Verifier SC).
// In development, LocalProvider simulates the full LNET stack.
type Provider interface {
	Name() string

	// --- Authentication ---

	// Login authenticates with username/password and returns access + refresh tokens.
	Login(ctx context.Context, req LoginRequest) (TokenResponse, error)

	// RefreshToken issues a new access token from a valid refresh token.
	RefreshToken(ctx context.Context, refreshToken string) (TokenResponse, error)

	// RevokeToken invalidates the given access token (logout).
	RevokeToken(ctx context.Context, accessToken string) error

	// ValidateToken parses and validates a token, returning enriched claims.
	ValidateToken(ctx context.Context, accessToken string) (TokenClaims, error)

	// --- Wallet / DID ---

	// CreateWallet provisions a DID + EVM wallet for the authenticated subject.
	// In production: D-Wallet API registers DID on-chain automatically.
	CreateWallet(ctx context.Context, accessToken string) (WalletResponse, error)

	// BindWallet associates a user identity with a specific wallet address.
	BindWallet(ctx context.Context, userID, walletAddress string) (WalletBinding, error)

	// GetByUser retrieves an existing wallet binding for a user.
	GetByUser(ctx context.Context, userID string) (WalletBinding, bool, error)

	// --- Signing ---

	// SignTransaction signs a 32-byte digest with the user's custody key.
	// In production: D-Wallet API signs; the private key never leaves LNET.
	SignTransaction(ctx context.Context, req SignRequest) (SignResponse, error)

	// --- KYC / Credentials ---

	// IssueKYCCredential creates a Verifiable Credential for a participant.
	// In production: SSI-VC-API issues a W3C VC and registers it on-chain.
	IssueKYCCredential(ctx context.Context, req KYCCredentialRequest) (KYCCredentialResponse, error)

	// VerifyKYCProof checks that a ZKP pointer corresponds to a valid credential.
	// In production: Claims Verifier smart contract on Besu.
	VerifyKYCProof(ctx context.Context, zkpPointer string) (bool, error)

	// GetKYCStatus returns the current KYC lifecycle status for a subject.
	GetKYCStatus(ctx context.Context, subject string) (KYCStatus, error)

	// ProvisionParticipant sets the KYC status for a subject (Central Bank only).
	// Used for approval, freeze, unfreeze, and revocation actions.
	ProvisionParticipant(ctx context.Context, subject string, status KYCStatus) error
}

// This file declares KYC read and write contracts used by compliance flows.
package interfaces

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

// KYCChecker defines read access to KYC status by subject.
// Used by WalletBind (sync check) and compliance handlers.
type KYCChecker interface {
	GetStatus(subject string) domain.KYCStatus
}

// KYCManager defines the full KYC lifecycle (provision, freeze, status).
// Implemented by IdentityGRPCManager which delegates to identity service.
type KYCManager interface {
	GetKYCStatus(ctx context.Context, subject string) (domain.KYCStatus, error)
	ProvisionParticipant(ctx context.Context, subject string, status domain.KYCStatus) error
}

// OnboardParticipantResult holds the response from the administrative
// onboarding flow (POST /compliance/register).
type OnboardParticipantResult struct {
	UserID        string
	WalletAddress string
	CertPEM       string
	TxHash        string
	// ClientSecret is the one-time secret generated at onboarding.
	// Must be stored by the caller; the server will not expose it again.
	ClientSecret string
}

// ParticipantOnboarder handles administrative onboarding of new participants
// (Commercial Banks, Treasury users) by the Central Bank.
type ParticipantOnboarder interface {
	OnboardParticipant(ctx context.Context, req OnboardParticipantRequest) (OnboardParticipantResult, error)
}

// UserSummary is a condensed participant view for list responses.
type UserSummary struct {
	UserID          string
	InstitutionName string
	Role            string
	Status          string
	WalletAddress   string
	Country         string
	BankCode        string
}

// UserDetail is the full profile of a single participant.
type UserDetail struct {
	UserID          string
	Username        string
	Email           string
	InstitutionName string
	Role            string
	Status          string
	WalletAddress   string
	Country         string
	BankCode        string
}

// UserManager allows the Central Bank to list and retrieve registered participants.
type UserManager interface {
	// ListUsers returns participants filtered by optional role and status.
	ListUsers(ctx context.Context, role, status string) ([]UserSummary, int, error)
	// GetUser returns the full profile of a single participant by their Keycloak UUID.
	GetUser(ctx context.Context, userID string) (UserDetail, error)
}

// OnboardParticipantRequest carries the payload for the administrative
// registration flow delegated to the identity gRPC service.
type OnboardParticipantRequest struct {
	Username        string
	Email           string
	Role            string
	InstitutionName string
	Country         string
	BankCode        string
}

// --- 3-Phase Onboarding (PKI + Blockchain) ---

// CredentialRequest carries the Phase 1 payload from a commercial bank.
type CredentialRequest struct {
	CsrPem              string
	BlockchainPubKeyHex string
	InstitutionName     string
	CNPJ                string
	BankCode            string
	Country             string
	Role                string
	Email               string
	Username            string
}

// CredentialRequestResult is the Phase 1 response.
type CredentialRequestResult struct {
	RequestID     string
	UserID        string
	WalletAddress string
	Status        string
}

// OnboardingStatus is the Phase 2.5 polling response.
type OnboardingStatus struct {
	RequestID     string
	UserID        string
	Status        string
	PopNonce      string
	WalletAddress string
}

// CompleteOnboardingRequest is the Phase 3 payload.
type CompleteOnboardingRequest struct {
	RequestID           string
	UserID              string
	PopSignatureHex     string
	BlockchainPubKeyHex string
}

// CompleteOnboardingResult is the Phase 3 response.
type CompleteOnboardingResult struct {
	UserID        string
	WalletAddress string
	CertPEM       string
	TxHash        string
	ClientSecret  string
	Status        string
}

// OnboardingManager handles the 3-phase PKI+Blockchain onboarding flow.
type OnboardingManager interface {
	SubmitCredentialRequest(ctx context.Context, req CredentialRequest) (CredentialRequestResult, error)
	GetOnboardingStatus(ctx context.Context, requestID string) (OnboardingStatus, error)
	CompleteOnboarding(ctx context.Context, req CompleteOnboardingRequest) (CompleteOnboardingResult, error)
}

// OnboardingKeyManager provides KMS operations for the commercial bank proxy.
// The proxy uses these to generate blockchain keys and sign PoP nonces
// internally, removing the burden from the frontend.
type OnboardingKeyManager interface {
	// CreateOnboardingKey generates (or retrieves) a secp256k1 key for bankCode.
	CreateOnboardingKey(ctx context.Context, bankCode string) (pubKeyHex, address string, err error)
	// SignOnboardingPoP signs the PoP nonce with the KMS key for keyID.
	// Returns the signature (V=0/1) and the public key hex.
	SignOnboardingPoP(ctx context.Context, keyID, nonceHex string) (signatureHex, pubKeyHex string, err error)
}

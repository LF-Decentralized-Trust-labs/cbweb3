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

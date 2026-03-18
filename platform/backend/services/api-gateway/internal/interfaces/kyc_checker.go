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

// ParticipantRegistrar handles participant onboarding via identity gRPC.
type ParticipantRegistrar interface {
	RegisterParticipant(ctx context.Context, accessToken, country, bankCode, role, institutionName string) (RegisterParticipantResult, error)
}

// RegisterParticipantResult holds the response from participant registration.
type RegisterParticipantResult struct {
	UserID        string
	DID           string
	WalletAddress string
}

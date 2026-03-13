// This file declares the KYC lookup contract used by auth and compliance flows.
package interfaces

import "github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"

// KYCChecker defines read access to KYC status by subject.
type KYCChecker interface {
	GetStatus(subject string) domain.KYCStatus
}


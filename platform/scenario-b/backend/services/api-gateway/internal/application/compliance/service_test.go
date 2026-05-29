// This file tests KYC status retrieval and updates in the compliance service.
package compliance

import (
	"testing"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

func TestComplianceServiceStatuses(t *testing.T) {
	t.Parallel()

	service := NewService(map[string]domain.KYCStatus{
		"bank-a": domain.KYCApproved,
	})

	if got := service.GetStatus("bank-a"); got != domain.KYCApproved {
		t.Fatalf("expected APPROVED, got %s", got)
	}
	if got := service.GetStatus("bank-x"); got != domain.KYCPending {
		t.Fatalf("expected PENDING, got %s", got)
	}

	service.SetStatus("bank-x", domain.KYCRejected)
	if got := service.GetStatus("bank-x"); got != domain.KYCRejected {
		t.Fatalf("expected REJECTED, got %s", got)
	}
}


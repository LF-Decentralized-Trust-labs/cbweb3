package ports

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
)

// HTLCRepository abstracts persistence for HTLC coordination records.
type HTLCRepository interface {
	CreateHTLC(ctx context.Context, record *domain.HTLCRecord) error
	GetHTLC(ctx context.Context, contractID string) (*domain.HTLCRecord, error)
	GetHTLCByHashLock(ctx context.Context, hashLock string) (*domain.HTLCRecord, error)
	UpdateHTLC(ctx context.Context, record *domain.HTLCRecord) error
	ListHTLCs(ctx context.Context, filter HTLCFilter) ([]*domain.HTLCRecord, error)
	ListNonTerminal(ctx context.Context) ([]*domain.HTLCRecord, error)
}

// HTLCFilter restricts ListHTLCs results.
type HTLCFilter struct {
	AgreementID string
	Sender      string
	Receiver    string
	State       string
}

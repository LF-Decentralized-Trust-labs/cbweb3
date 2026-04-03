package ports

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
)

// EscrowRepository abstracts persistence for the escrow flow records.
type EscrowRepository interface {
	CreateDeposit(ctx context.Context, record domain.DepositRecord) error
	GetDeposit(ctx context.Context, id string) (domain.DepositRecord, bool, error)
	UpdateDeposit(ctx context.Context, record domain.DepositRecord) error
	ListDeposits(ctx context.Context, requesterID string) ([]domain.DepositRecord, error)

	CreateEscrow(ctx context.Context, record domain.EscrowRecord) error
	GetEscrow(ctx context.Context, id string) (domain.EscrowRecord, bool, error)
	UpdateEscrow(ctx context.Context, record domain.EscrowRecord) error
	ListEscrows(ctx context.Context, requesterID string) ([]domain.EscrowRecord, error)

	CreateRedeem(ctx context.Context, record domain.RedeemRecord) error
	GetRedeem(ctx context.Context, id string) (domain.RedeemRecord, bool, error)
	UpdateRedeem(ctx context.Context, record domain.RedeemRecord) error
	ListRedeems(ctx context.Context, requesterID string) ([]domain.RedeemRecord, error)
}

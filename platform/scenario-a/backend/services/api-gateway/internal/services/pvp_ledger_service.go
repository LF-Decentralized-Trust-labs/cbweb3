// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"fmt"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"gorm.io/gorm"
)

// SettledLegInput is a settled PvP leg reported by an orchestrator. ReceiverBankID
// is pre-parsed by the caller (which owns the identity-format knowledge).
type SettledLegInput struct {
	ContractID     string
	TradeID        string
	Sender         string
	Receiver       string
	ReceiverBankID string
	Amount         string
	SettledAt      time.Time
}

// PvPCreditRow is one incoming settled leg for a bank, as served to the statement.
type PvPCreditRow struct {
	Reference string // HTLC contract id (unique per leg; shared with the sender's debit)
	Amount    string
	SettledAt string // RFC3339
}

// PvPLedgerService persists settled inter-bank PvP legs at the Central Bank and
// serves each bank the legs on which it is the receiver (its credit side).
type PvPLedgerService struct {
	db *gorm.DB
}

// NewPvPLedgerService creates a PvPLedgerService over the given DB.
func NewPvPLedgerService(db *gorm.DB) *PvPLedgerService {
	return &PvPLedgerService{db: db}
}

// RecordLeg upserts a settled leg keyed by ContractID, so settle retries or relay
// redelivery of the same leg do not duplicate a movement.
func (s *PvPLedgerService) RecordLeg(ctx context.Context, in SettledLegInput) error {
	if s.db == nil {
		return fmt.Errorf("pvp ledger: no database configured")
	}
	if in.ContractID == "" || in.ReceiverBankID == "" {
		return fmt.Errorf("pvp ledger: contract_id and receiver bank are required")
	}
	leg := domain.PvPSettledLeg{
		ContractID:     in.ContractID,
		TradeID:        in.TradeID,
		Sender:         in.Sender,
		Receiver:       in.Receiver,
		ReceiverBankID: in.ReceiverBankID,
		Amount:         in.Amount,
		SettledAt:      in.SettledAt,
	}
	// Save upserts by primary key (ContractID): insert if new, update if re-reported.
	if err := s.db.WithContext(ctx).Save(&leg).Error; err != nil {
		return fmt.Errorf("pvp ledger: save leg: %w", err)
	}
	return nil
}

// ListCreditsForBank returns the settled legs on which bankID is the receiver,
// most recent first.
func (s *PvPLedgerService) ListCreditsForBank(ctx context.Context, bankID string) ([]PvPCreditRow, error) {
	if s.db == nil {
		return nil, fmt.Errorf("pvp ledger: no database configured")
	}
	var legs []domain.PvPSettledLeg
	if err := s.db.WithContext(ctx).
		Where("receiver_bank_id = ?", bankID).
		Order("settled_at desc").
		Find(&legs).Error; err != nil {
		return nil, fmt.Errorf("pvp ledger: list credits: %w", err)
	}
	rows := make([]PvPCreditRow, 0, len(legs))
	for _, l := range legs {
		settledAt := ""
		if !l.SettledAt.IsZero() {
			settledAt = l.SettledAt.UTC().Format(time.RFC3339)
		}
		rows = append(rows, PvPCreditRow{
			Reference: l.ContractID,
			Amount:    l.Amount,
			SettledAt: settledAt,
		})
	}
	return rows, nil
}

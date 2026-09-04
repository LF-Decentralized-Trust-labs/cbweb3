// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"gorm.io/gorm"
)

// SettledLegInput is a settled PvP leg reported by an orchestrator.
//
// Receiver is the authoritative field: it is the receiving party's Paladin identity,
// exactly as the settling orchestrator reported it. ReceiverBankID is a BEST-EFFORT
// label kept only to narrow queries; it is not required and must never be the basis
// of a scoping decision. A bank id cannot be recovered from an identity by splitting
// on "-" (see identityBelongsToBank in the handlers package), and a leg that already
// settled must not be refused because a label could not be derived.
type SettledLegInput struct {
	ContractID     string
	TradeID        string
	Sender         string
	Receiver       string
	ReceiverBankID string // best-effort label; may be empty or wrong. Do not scope on it.
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
	// Validate the authoritative fields only. Requiring ReceiverBankID here is what
	// let a derivation failure discard an already-settled leg: the reporting
	// orchestrator only logs a warning and never retries, so a rejection was a
	// permanent hole in the ledger.
	if in.ContractID == "" || strings.TrimSpace(in.Receiver) == "" {
		return fmt.Errorf("pvp ledger: contract_id and receiver are required")
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
//
// Scoping TESTS the stored receiver identity; it does not compare a derived bank id.
// The previous `receiver_bank_id = ?` match was wrong whenever the writer's split
// disagreed with the bank's configured code, which is exactly what happens when a
// spoke id contains a hyphen:
//
//	receiver funded_operator@spoke-costa-rica-cb1  stored label "rica-cb1"
//	BANK_CODE (manifest spec.bankId)               "cb1"          -> zero rows
//
// A Costa Rica bank therefore received value on-chain and saw no credit on its
// statement. Chile, Peru and the samples' hyphenated bank ids matched by accident.
//
// belongs is the authority (the caller passes the same predicate used for
// authorization) and the SQL below is only a narrowing pre-filter: it must return a
// SUPERSET of what belongs accepts, so any drift between the two is safe — SQL
// over-fetches and Go rejects. Never let the SQL become the decision.
//
// No backfill is needed: Receiver was always stored correctly, and only the derived
// label was wrong. Rows written before this fix are scoped correctly from here on.
func (s *PvPLedgerService) ListCreditsForBank(ctx context.Context, bankID string, belongs func(identity string) bool) ([]PvPCreditRow, error) {
	if s.db == nil {
		return nil, fmt.Errorf("pvp ledger: no database configured")
	}
	bankID = strings.TrimSpace(bankID)
	if bankID == "" || belongs == nil {
		// An unscoped read of the CB's ledger would hand one bank every other bank's
		// settlements. Fail closed.
		return nil, fmt.Errorf("pvp ledger: list credits: a bank scope and a membership predicate are required")
	}
	var legs []domain.PvPSettledLeg
	pat := CreditScopePatterns(bankID)
	if err := s.db.WithContext(ctx).
		Where("receiver LIKE ? OR receiver LIKE ?", pat[0], pat[1]).
		Order("settled_at desc").
		Find(&legs).Error; err != nil {
		return nil, fmt.Errorf("pvp ledger: list credits: %w", err)
	}
	rows := make([]PvPCreditRow, 0, len(legs))
	for _, l := range legs {
		if !belongs(l.Receiver) {
			continue // SQL over-fetched (e.g. "-cb11" caught by a "%-cb1" pattern)
		}
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

// CreditScopePatterns builds the SQL LIKE patterns that narrow the credit query to
// one bank. It mirrors the two shapes the membership predicate accepts: the
// identity's node part either equals bankID, or ends at a "-" boundary before it.
// Identities are stored as "<key>@<node>", so a suffix match on the whole string
// covers both ("…@cb1" and "…-cb1").
//
// These patterns MUST return a superset of what the predicate accepts — they narrow
// the scan, they do not decide. ListCreditsForBank applies the predicate to every
// row that comes back, so a pattern that is too broad is harmless (an over-fetched
// row is rejected in Go) while one that is too narrow silently loses a credit. When
// in doubt, widen. A bankID containing a LIKE wildcard only widens the match, which
// is the safe direction.
//
// Exported so the handlers package — where the predicate lives — can assert the
// superset property over the shared case table. That test is the drift guard.
func CreditScopePatterns(bankID string) [2]string {
	bankID = strings.TrimSpace(bankID)
	return [2]string{"%@" + bankID, "%-" + bankID}
}

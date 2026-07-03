// SPDX-License-Identifier: Apache-2.0

package ports

import (
	"context"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
)

// FXAgreementFilter specifies filtering options for ListAgreements.
type FXAgreementFilter struct {
	// Counterparty matches either Originator or CounterpartyB.
	Counterparty string
	// State filters by exact FXState value. Empty means all states.
	State domain.FXState
}

// FXAgreementRepository provides durable storage for FX agreements and audit events.
// Implementations must guarantee:
//   - Idempotent CreateAgreement (unique trade_id).
//   - Append-only CreateAuditEvent (events are never updated or deleted).
//   - Terminal states (REJECTED, CANCELLED, SETTLED) cannot be overwritten.
type FXAgreementRepository interface {
	// CreateAgreement persists a new FX agreement record.
	// Idempotent on TradeID: a duplicate is a silent no-op, not an error (the synchronous
	// propose handler and the FXIndexer can race to project the same on-chain agreement).
	CreateAgreement(ctx context.Context, r *domain.FXAgreementRecord) error

	// GetAgreement retrieves an FX agreement by trade_id.
	// Returns (nil, nil) when not found.
	GetAgreement(ctx context.Context, tradeID string) (*domain.FXAgreementRecord, error)

	// UpdateAgreement saves state and timestamp changes to an existing record.
	UpdateAgreement(ctx context.Context, r *domain.FXAgreementRecord) error

	// ListAgreements returns agreements matching the filter. Ordered by created_at DESC.
	ListAgreements(ctx context.Context, f FXAgreementFilter) ([]*domain.FXAgreementRecord, error)

	// CreateAuditEvent appends an immutable lifecycle event for the given trade_id.
	CreateAuditEvent(ctx context.Context, e *domain.FXAgreementEvent) error

	// ListAuditEvents returns all events for a trade_id, ordered by occurred_at ASC.
	ListAuditEvents(ctx context.Context, tradeID string) ([]*domain.FXAgreementEvent, error)

	// ListExpiredNonTerminal returns all agreements whose expiry_date is before the
	// provided unix-seconds timestamp and whose state is not terminal.
	ListExpiredNonTerminal(ctx context.Context, nowUnix int64) ([]*domain.FXAgreementRecord, error)
}

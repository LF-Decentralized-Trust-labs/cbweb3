// SPDX-License-Identifier: Apache-2.0

// Package app provides CrossCurrencySwapRepository for persisting swap operations.
//
// Feature: 009-commercial-cross-currency-swap
// Spec: specs/009-commercial-cross-currency-swap/data-model.md
package app

import (
	"context"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// crossCurrencySwapRepository persists CrossCurrencySwapOperation records.
type crossCurrencySwapRepository struct {
	db *gorm.DB
}

// newCrossCurrencySwapRepository creates a crossCurrencySwapRepository backed by the given DB.
func newCrossCurrencySwapRepository(db *gorm.DB) *crossCurrencySwapRepository {
	return &crossCurrencySwapRepository{db: db}
}

// Create inserts a new CrossCurrencySwapOperation record.
func (r *crossCurrencySwapRepository) Create(ctx context.Context, op *domain.CrossCurrencySwapOperation) error {
	return r.db.WithContext(ctx).Create(op).Error
}

// GetByID retrieves a CrossCurrencySwapOperation by swap_id (satisfies CrossCurrencySwapRepository).
func (r *crossCurrencySwapRepository) GetByID(ctx context.Context, swapID string) (*domain.CrossCurrencySwapOperation, error) {
	var op domain.CrossCurrencySwapOperation
	err := r.db.WithContext(ctx).Where("swap_id = ?", swapID).First(&op).Error
	if err != nil {
		return nil, err
	}
	return &op, nil
}

// FindByID is an alias for GetByID for internal callers.
func (r *crossCurrencySwapRepository) FindByID(ctx context.Context, swapID string) (*domain.CrossCurrencySwapOperation, error) {
	return r.GetByID(ctx, swapID)
}

// FindByCorrelationID retrieves all CrossCurrencySwapOperation records by correlation_id.
// Useful for auditing all swap attempts linked to a single correlation ID.
func (r *crossCurrencySwapRepository) FindByCorrelationID(ctx context.Context, correlationID string) ([]domain.CrossCurrencySwapOperation, error) {
	var ops []domain.CrossCurrencySwapOperation
	err := r.db.WithContext(ctx).Where("correlation_id = ?", correlationID).Find(&ops).Error
	return ops, err
}

// ListByPayer returns a page of swap operations initiated by payerBankID, newest first,
// optionally bounded by [from, to] on created_at (nil = unbounded on that side). It also
// returns the total row count matching the filter (for pagination). limit<=0 defaults to
// 20; offset<0 becomes 0.
func (r *crossCurrencySwapRepository) ListByPayer(
	ctx context.Context,
	payerBankID string,
	from, to *time.Time,
	limit, offset int,
) ([]domain.CrossCurrencySwapOperation, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	q := r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("payer_bank_id = ?", payerBankID)
	if from != nil {
		q = q.Where("created_at >= ?", *from)
	}
	if to != nil {
		q = q.Where("created_at <= ?", *to)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var ops []domain.CrossCurrencySwapOperation
	if err := q.Order("created_at DESC").Limit(limit).Offset(offset).Find(&ops).Error; err != nil {
		return nil, 0, err
	}
	return ops, total, nil
}

// UpdateStatus updates the status field of a CrossCurrencySwapOperation.
func (r *crossCurrencySwapRepository) UpdateStatus(ctx context.Context, swapID string, status domain.SwapOperationStatus) error {
	return r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("swap_id = ?", swapID).
		Update("status", status).Error
}

// UpdateBridgeInPositionID persists the bridge-in position ID after the relayer activates it.
func (r *crossCurrencySwapRepository) UpdateBridgeInPositionID(ctx context.Context, swapID string, positionID string) error {
	return r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("swap_id = ?", swapID).
		Update("bridge_in_position_id", positionID).Error
}

// UpdateSwapResult atomically persists the swap tx hash and the realized amount_in after the
// AMM swap executes on Hub. Both columns are written in a single UPDATE so a crash cannot leave
// the record with a tx hash but a stale/empty amount_in (which now holds the real cost decoded
// from LogSwap, not the MaxAmountIn cap).
func (r *crossCurrencySwapRepository) UpdateSwapResult(ctx context.Context, swapID string, txHash string, amountIn string) error {
	return r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("swap_id = ?", swapID).
		Updates(map[string]interface{}{
			"swap_tx_hash": txHash,
			"amount_in":    amountIn,
		}).Error
}

// UpdateBridgeOutPositionID updates the bridge_out_position_id field after bridge-out initiated.
func (r *crossCurrencySwapRepository) UpdateBridgeOutPositionID(ctx context.Context, swapID string, positionID string) error {
	return r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("swap_id = ?", swapID).
		Update("bridge_out_position_id", positionID).Error
}

// UpdateResidue persists the outcome of the residue return (MaxAmountIn − realized amount_in)
// in a single UPDATE. It deliberately does not touch `status`: the payment is already final
// when the residue is handled, so a failed return records RETURN_FAILED for reconciliation
// without reopening a COMPLETED swap.
func (r *crossCurrencySwapRepository) UpdateResidue(ctx context.Context, swapID, amount, positionID string, status domain.ResidueReturnStatus) error {
	updates := map[string]interface{}{
		"residue_amount": amount,
		"residue_status": status,
	}
	if positionID != "" {
		updates["residue_position_id"] = positionID
	}
	return r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("swap_id = ?", swapID).
		Updates(updates).Error
}

// ClaimNextRetryableResidue returns the OLDEST swap whose residue return failed to ENQUEUE and
// whose next attempt is due — and claims it so a concurrent sweeper skips it. It returns
// (nil, nil) when nothing is due.
//
// One row per call, on purpose. Claiming a batch and leasing all of it at once ties the lease
// to the batch size: a sweeper working through 50 rows at up to one dispatch timeout each needs
// far longer than a lease sized for a single dispatch, and once it lapses another sweeper claims
// the rows this one has not reached yet. Claiming per row writes each lease immediately before
// the dispatch it covers, so the lease only ever has to cover one call and the sweep's batch
// limit is free to be whatever the operator wants.
//
// Only RETURN_FAILED is retryable here. RETURN_ENQUEUED already has a bridge position and is
// driven by the relayer's own queue; NONE has nothing to return; RETURN_ESCALATED gave up and
// needs a human. A NULL next_attempt_at is due immediately — that is the state a first failure
// leaves behind, since it predates any scheduling.
//
// The claim is two statements in ONE SHORT transaction, and deliberately does not span the
// work:
//
//   - SELECT … FOR UPDATE SKIP LOCKED picks rows no other sweeper is claiming right now. It is
//     Postgres-only; SQLite (tests) has a single writer and rejects the clause.
//   - the same transaction writes a lease into residue_next_attempt_at, which is what keeps the
//     rows hidden AFTER the transaction commits.
//
// Holding the row locks across the sweep instead would be simpler to write and wrong to run:
// retryOne dispatches to the issuing CB over the network, so the transaction — and a pooled
// connection — would stay open for the length of an HTTP call to another entity, for every row.
// The lease gives the same exclusion without holding anything.
//
// The lease consumes no attempt. Every normal outcome (success, failure, breaker deferral)
// writes a real schedule over it, so it only decides how long a residue waits when a sweep dies
// mid-row.
// withResidueClaimLock adds FOR UPDATE SKIP LOCKED where the database supports it.
//
// Postgres-only by necessity, not preference: SQLite (the repository tests) has a single writer
// and rejects the clause, so an ungated version would fail every one of those tests instead of
// failing in production. Splitting the decision out makes it testable without a server — the
// emitted SQL is verified against a real Postgres separately.
func withResidueClaimLock(q *gorm.DB, dialect string) *gorm.DB {
	if dialect != "postgres" {
		return q
	}
	return q.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
}

func (r *crossCurrencySwapRepository) ClaimNextRetryableResidue(ctx context.Context, now time.Time) (*domain.CrossCurrencySwapOperation, error) {
	var ops []domain.CrossCurrencySwapOperation
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := tx.
			Where("residue_status = ?", domain.ResidueReturnFailed).
			// The attempt ceiling is enforced by the QUERY, not only by the status write that
			// escalates a row. Those are two separate statements: if the status write fails
			// while the counter write succeeds, the row stays RETURN_FAILED at the ceiling, and
			// without this bound it would be re-dispatched to the issuing CB on every sweep
			// forever.
			Where("residue_attempts < ?", services.ResidueMaxAttempts()).
			Where("residue_next_attempt_at IS NULL OR residue_next_attempt_at <= ?", now).
			Order("created_at ASC").
			Limit(1)
		q = withResidueClaimLock(q, tx.Dialector.Name())
		if err := q.Find(&ops).Error; err != nil {
			return err
		}
		if len(ops) == 0 {
			return nil
		}
		lease := now.Add(services.ResidueClaimLease())
		return tx.Model(&domain.CrossCurrencySwapOperation{}).
			Where("swap_id = ?", ops[0].SwapID).
			Update("residue_next_attempt_at", lease).Error
	})
	if err != nil {
		return nil, err
	}
	if len(ops) == 0 {
		return nil, nil
	}
	return &ops[0], nil
}

// RecordResidueAttempt persists the attempt counter and when the next attempt becomes due.
// A nil nextAttemptAt clears the schedule, which is what a terminal outcome (enqueued or
// escalated) leaves behind.
//
// It also clears residue_deferred_since. Recording an attempt means the row was actually tried,
// so any deferral window is over; leaving a stale stamp behind would make a pause months later
// escalate on its very first sweep.
func (r *crossCurrencySwapRepository) RecordResidueAttempt(ctx context.Context, swapID string, attempts int, nextAttemptAt *time.Time) error {
	return r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("swap_id = ?", swapID).
		Updates(map[string]interface{}{
			"residue_attempts":        attempts,
			"residue_next_attempt_at": nextAttemptAt,
			"residue_deferred_since":  nil,
		}).Error
}

// DeferResidue reschedules a row whose pair is halted by governance without touching the attempt
// counter — deferring is not an attempt — and opens a deferral window if none is open.
//
// The stamp is written with COALESCE rather than read-then-write: two sweepers can look at the
// same row across a claim lease boundary, and a read-modify-write would let the later one move the
// start of the pause forward. Every move forward pushes the bound further away, which is precisely
// the unbounded wait this column exists to end.
func (r *crossCurrencySwapRepository) DeferResidue(ctx context.Context, swapID string, nextAttemptAt, deferredSince time.Time) error {
	return r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("swap_id = ?", swapID).
		Updates(map[string]interface{}{
			"residue_next_attempt_at": nextAttemptAt,
			"residue_deferred_since":  gorm.Expr("COALESCE(residue_deferred_since, ?)", deferredSince),
		}).Error
}

// withBridgeOutClaimLock adds FOR UPDATE SKIP LOCKED where the database supports it. Same
// necessity as withResidueClaimLock: SQLite (the repository tests) has a single writer and
// rejects the clause, so an ungated version would fail every one of those tests rather than
// failing in production.
func withBridgeOutClaimLock(q *gorm.DB, dialect string) *gorm.DB {
	if dialect != "postgres" {
		return q
	}
	return q.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
}

// ClaimNextRetryableBridgeOut picks the oldest delivery that failed and is due, and leases it.
//
// Only DELIVERY_FAILED is retryable. DELIVERY_NOTIFIED already has a position on CB-B and is
// driven by that CB's own relayer queue; DELIVERY_ESCALATED gave up and needs a human; an empty
// status is a legacy row that predates this column and carries no evidence either way, so it is
// left alone rather than re-delivered on a guess.
//
// A NULL next_attempt_at is due immediately, which is what a first failure would leave behind if
// the schedule write were ever lost.
//
// The claim is two statements in ONE SHORT transaction and deliberately does not span the work:
// the retry dispatches to the relay over the network, so holding row locks across it would keep
// a transaction — and a pooled connection — open for the length of an HTTP call to another
// entity, per row. The lease gives the same exclusion without holding anything.
func (r *crossCurrencySwapRepository) ClaimNextRetryableBridgeOut(ctx context.Context, now time.Time) (*domain.CrossCurrencySwapOperation, error) {
	var ops []domain.CrossCurrencySwapOperation
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := tx.
			Where("bridge_out_status = ?", domain.BridgeOutDeliveryFailed).
			// The ceiling is enforced by the QUERY, not only by the status write that escalates
			// a row. Those are separate statements: if the status write fails while the counter
			// write succeeds, the row stays DELIVERY_FAILED at the ceiling and without this
			// bound would be re-dispatched on every sweep forever.
			Where("bridge_out_attempts < ?", services.BridgeOutMaxAttempts()).
			Where("bridge_out_next_attempt_at IS NULL OR bridge_out_next_attempt_at <= ?", now).
			Order("created_at ASC").
			Limit(1)
		q = withBridgeOutClaimLock(q, tx.Dialector.Name())
		if err := q.Find(&ops).Error; err != nil {
			return err
		}
		if len(ops) == 0 {
			return nil
		}
		lease := now.Add(services.BridgeOutClaimLease())
		return tx.Model(&domain.CrossCurrencySwapOperation{}).
			Where("swap_id = ?", ops[0].SwapID).
			Update("bridge_out_next_attempt_at", lease).Error
	})
	if err != nil {
		return nil, err
	}
	if len(ops) == 0 {
		return nil, nil
	}
	return &ops[0], nil
}

// UpdateBridgeOutDelivery records the delivery status, the attempt counter and when the next
// attempt becomes due. A nil nextAttemptAt clears the schedule, which is what a terminal outcome
// (notified or escalated) leaves behind.
//
// It also clears bridge_out_deferred_since: recording an attempt means the row was actually
// tried, so any deferral window is over. Leaving a stale stamp would make a pause months later
// escalate on its very first sweep.
//
// It does NOT touch `status`. The swap's own verdict was set when the delivery first failed, and
// a delivery that later succeeds must not silently rewrite a record the payer has already been
// shown — the recovery is visible in bridge_out_status, which is the field that is actually
// about the delivery.
func (r *crossCurrencySwapRepository) UpdateBridgeOutDelivery(ctx context.Context, swapID string, status domain.BridgeOutDeliveryStatus, attempts int, nextAttemptAt *time.Time) error {
	return r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("swap_id = ?", swapID).
		Updates(map[string]interface{}{
			"bridge_out_status":          status,
			"bridge_out_attempts":        attempts,
			"bridge_out_next_attempt_at": nextAttemptAt,
			"bridge_out_deferred_since":  nil,
		}).Error
}

// DeferBridgeOut reschedules a row whose pair is halted by governance without touching the
// attempt counter — deferring is not an attempt — and opens a deferral window if none is open.
//
// The stamp is written with COALESCE rather than read-then-write: two sweepers can look at the
// same row across a claim lease boundary, and a read-modify-write would let the later one move
// the start of the pause forward. Every move forward pushes the bound further away, which is
// precisely the unbounded wait this column exists to end.
func (r *crossCurrencySwapRepository) DeferBridgeOut(ctx context.Context, swapID string, nextAttemptAt, deferredSince time.Time) error {
	return r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("swap_id = ?", swapID).
		Updates(map[string]interface{}{
			"bridge_out_next_attempt_at": nextAttemptAt,
			"bridge_out_deferred_since":  gorm.Expr("COALESCE(bridge_out_deferred_since, ?)", deferredSince),
		}).Error
}

// MarkDeliveredAfterRetry records a recovered delivery: the reason a person reads and the
// verdict they read first, in one statement so no window exists where the two contradict.
//
// Guarded on status = FAILED. The only row this may promote is one the synchronous call gave
// up on; a COMPLETED row reaching here would mean the delivery succeeded twice, and rewriting
// its verdict would be a regression, not a repair.
func (r *crossCurrencySwapRepository) MarkDeliveredAfterRetry(ctx context.Context, swapID string, reason string) error {
	return r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("swap_id = ? AND status = ?", swapID, domain.SwapStatusFailed).
		Updates(map[string]interface{}{
			"failure_reason": reason,
			"status":         domain.SwapStatusDeliveredAfterRetry,
		}).Error
}

// UpdateFailureReason sets the failure_reason field when status=FAILED.
func (r *crossCurrencySwapRepository) UpdateFailureReason(ctx context.Context, swapID string, reason string) error {
	return r.db.WithContext(ctx).
		Model(&domain.CrossCurrencySwapOperation{}).
		Where("swap_id = ?", swapID).
		Updates(map[string]interface{}{
			"failure_reason": reason,
			"status":         domain.SwapStatusFailed,
		}).Error
}

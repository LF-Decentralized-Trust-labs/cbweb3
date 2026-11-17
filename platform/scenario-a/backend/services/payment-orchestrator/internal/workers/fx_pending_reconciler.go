// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

// PendingFXStore is the slice of the FX repository the reconciler uses.
type PendingFXStore interface {
	ListPendingAgreements(ctx context.Context, limit int) ([]*domain.FXAgreementRecord, error)
	UpdateAgreement(ctx context.Context, r *domain.FXAgreementRecord) error
	CreateAuditEvent(ctx context.Context, e *domain.FXAgreementEvent) error
}

// FXPendingReconciler brings agreements with a pending on-chain operation back in line with the
// chain once that operation resolves.
//
// An operation is recorded as pending when it was, or may have been, accepted by Paladin but its
// outcome is unknown (ErrPrivateTxUnconfirmed). On Paladin v1.0.0 such a transaction can stay
// pending indefinitely and cannot be cancelled (upstream LFDT-Paladin/paladin#1356), and it may
// still commit much later. Until it resolves, the server refuses any other operation on the
// agreement. Each cycle, for every pending record, the reconciler:
//
//   - asks Paladin for the transaction's receipt, when its id is known;
//   - on a failed receipt, clears the pending operation: it did not happen, and the agreement
//     can be operated on again;
//   - on a successful receipt, or (when no id is known) once the chain shows the operation's
//     target state, applies the on-chain state to the record and clears the pending operation;
//   - otherwise leaves it pending, and warns once per warnEvery while it stays pending longer
//     than warnAfter, so an operator can look at the Paladin node.
//
// The chain is authoritative. A record already in a terminal state is not overwritten; its
// pending operation is cleared with a warning.
type FXPendingReconciler struct {
	store     PendingFXStore
	chain     ports.FXPendingChainPort
	interval  time.Duration
	warnAfter time.Duration
	warnEvery time.Duration
	batch     int
	logger    *slog.Logger
	lastWarn  map[string]time.Time
	now       func() time.Time
}

// NewFXPendingReconciler builds a reconciler. A non-positive interval means 30 s; warnAfter is
// how long an operation may stay pending before it is reported (non-positive means 10 min).
func NewFXPendingReconciler(store PendingFXStore, chain ports.FXPendingChainPort, interval, warnAfter time.Duration, logger *slog.Logger) *FXPendingReconciler {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if warnAfter <= 0 {
		warnAfter = 10 * time.Minute
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &FXPendingReconciler{
		store: store, chain: chain, interval: interval, warnAfter: warnAfter,
		warnEvery: time.Hour, batch: ports.MaxFXAgreementPageSize, logger: logger,
		lastWarn: make(map[string]time.Time), now: time.Now,
	}
}

// Start runs the reconciler until ctx is cancelled.
func (w *FXPendingReconciler) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.logger.Info("fx pending-operation reconciler started", "interval", w.interval.String())
	w.RunOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("fx pending-operation reconciler stopped")
			return
		case <-ticker.C:
			w.RunOnce(ctx)
		}
	}
}

// RunOnce reconciles every pending record once.
func (w *FXPendingReconciler) RunOnce(ctx context.Context) {
	records, err := w.store.ListPendingAgreements(ctx, w.batch)
	if err != nil {
		w.logger.Error("fx pending reconciler: listing pending agreements failed", "error", err.Error())
		return
	}
	for _, rec := range records {
		if ctx.Err() != nil {
			return
		}
		w.reconcile(ctx, rec)
	}
}

func (w *FXPendingReconciler) reconcile(ctx context.Context, rec *domain.FXAgreementRecord) {
	p := rec.Pending
	log := w.logger.With("trade_id", rec.TradeID, "operation", string(p.Op), "paladin_tx_id", p.TxID)

	if rec.State.IsTerminal() {
		log.Warn("fx pending reconciler: clearing a pending operation on an agreement already in a terminal state",
			"state", string(rec.State))
		w.clear(ctx, rec, rec.State, fmt.Sprintf("pending %s cleared: agreement already %s", p.Op, rec.State))
		return
	}

	outcome := ports.PrivateTxPending
	if p.TxID != "" {
		o, err := w.chain.PrivateTxOutcome(ctx, p.TxID)
		if err != nil {
			log.Warn("fx pending reconciler: could not read the transaction outcome; will retry", "error", err.Error())
			return
		}
		outcome = o
	}

	switch outcome {
	case ports.PrivateTxFailed:
		log.Info("fx pending reconciler: the pending operation failed on chain; the agreement can be operated on again")
		w.clear(ctx, rec, rec.State, fmt.Sprintf("pending %s failed on chain (Paladin transaction %s)", p.Op, p.TxID))
		return
	case ports.PrivateTxPending:
		if p.TxID != "" {
			w.warnIfOld(rec, log)
			return
		}
		// No transaction id is known (the submission got no answer): only the chain can tell
		// whether it happened.
	}

	chainState, found, err := w.chainState(ctx, rec)
	if err != nil {
		log.Warn("fx pending reconciler: could not read the agreement from chain; will retry", "error", err.Error())
		return
	}
	if outcome == ports.PrivateTxPending && (!found || chainState != p.Op.TargetState()) {
		w.warnIfOld(rec, log)
		return
	}
	if !found {
		log.Error("fx pending reconciler: the transaction succeeded but the agreement is not on chain; reconcile manually")
		return
	}
	log.Info("fx pending reconciler: applying the on-chain state", "from", string(rec.State), "to", string(chainState))
	w.clear(ctx, rec, chainState, fmt.Sprintf("pending %s confirmed on chain", p.Op))
}

// chainState reads the agreement's on-chain state. found is false when it is not on chain.
func (w *FXPendingReconciler) chainState(ctx context.Context, rec *domain.FXAgreementRecord) (domain.FXState, bool, error) {
	tctx := ports.WithPenteFXTarget(ctx, ports.PenteFXTarget{GroupID: rec.GroupID, ContractAddress: rec.ContractAddress})
	id := domain.OnChainTradeID(rec.TradeID)
	st, err := w.chain.GetFXAgreement(tctx, "0x"+hex.EncodeToString(id[:]))
	if err != nil {
		return "", false, err
	}
	if st == nil {
		return "", false, nil
	}
	return domain.FXState(strings.TrimPrefix(st.State, "FX_STATE_")), true, nil
}

// clear sets the record's state, removes the pending operation, and appends an audit event.
func (w *FXPendingReconciler) clear(ctx context.Context, rec *domain.FXAgreementRecord, to domain.FXState, note string) {
	from := rec.State
	txID := rec.Pending.TxID
	rec.State = to
	rec.Pending = domain.FXPending{}
	rec.UpdatedAt = w.now().UTC()
	if err := w.store.UpdateAgreement(ctx, rec); err != nil {
		w.logger.Error("fx pending reconciler: update failed; will retry", "trade_id", rec.TradeID, "error", err.Error())
		return
	}
	delete(w.lastWarn, rec.TradeID)
	if err := w.store.CreateAuditEvent(ctx, &domain.FXAgreementEvent{
		TradeID: rec.TradeID, FromState: from, ToState: to, Actor: "fx-pending-reconciler",
		OccurredAt: w.now().UTC(), Notes: note, TxHash: txID, Source: domain.EventSourceSystemJob,
	}); err != nil {
		w.logger.Error("fx pending reconciler: audit event failed", "trade_id", rec.TradeID, "error", err.Error())
	}
}

func (w *FXPendingReconciler) warnIfOld(rec *domain.FXAgreementRecord, log *slog.Logger) {
	age := w.now().Sub(rec.Pending.Since)
	if rec.Pending.Since.IsZero() || age < w.warnAfter {
		return
	}
	if last, ok := w.lastWarn[rec.TradeID]; ok && w.now().Sub(last) < w.warnEvery {
		return
	}
	w.lastWarn[rec.TradeID] = w.now()
	log.Warn("fx pending reconciler: an FX operation has been pending for a long time; check the Paladin node",
		"pending_for", age.Round(time.Second).String())
}

// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"log/slog"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/domain"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/payment-orchestrator/internal/ports"
)

// FXExpirationWorker periodically cancels expired non-terminal agreements.
type FXExpirationWorker struct {
	repo     ports.FXAgreementRepository
	interval time.Duration
	logger   *slog.Logger
}

func NewFXExpirationWorker(repo ports.FXAgreementRepository, interval time.Duration, logger *slog.Logger) *FXExpirationWorker {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &FXExpirationWorker{repo: repo, interval: interval, logger: logger}
}

// Start runs the worker loop until ctx is cancelled.
func (w *FXExpirationWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("fx expiration worker stopped")
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *FXExpirationWorker) runOnce(ctx context.Context) {
	nowUnix := time.Now().Unix()
	records, err := w.repo.ListExpiredNonTerminal(ctx, nowUnix)
	if err != nil {
		w.logger.Error("fx expiration worker list failed", "error", err)
		return
	}
	for _, rec := range records {
		from := rec.State
		if from.IsTerminal() {
			continue
		}
		rec.State = domain.FXStateCancelled
		rec.UpdatedAt = time.Now().UTC()
		if err := w.repo.UpdateAgreement(ctx, rec); err != nil {
			w.logger.Error("fx expiration worker update failed", "trade_id", rec.TradeID, "error", err)
			continue
		}
		_ = w.repo.CreateAuditEvent(ctx, &domain.FXAgreementEvent{
			TradeID:    rec.TradeID,
			FromState:  from,
			ToState:    domain.FXStateCancelled,
			Actor:      "system:fx-expiration-worker",
			OccurredAt: time.Now().UTC(),
			Notes:      "automatic cancellation due to expiry",
			Source:     domain.EventSourceSystemJob,
		})
		w.logger.Info("fx agreement auto-cancelled by expiry", "trade_id", rec.TradeID, "from_state", from)
	}
}

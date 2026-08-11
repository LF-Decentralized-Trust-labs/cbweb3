// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
)

// reconciliationDefaultInterval is how often the Hub balance is reconciled against the records.
// Unhurried on purpose: it is one chain read plus a few indexed queries, and the condition it
// looks for does not appear and vanish within minutes.
const reconciliationDefaultInterval = 5 * time.Minute

// startHubReconciliationChecker periodically reconciles this CB's Hub W-token balance and emits a
// structured line when it does not balance.
//
// Why a periodic check and not only an endpoint: an endpoint nobody calls detects nothing. The
// alert is the deliverable — the endpoint and any dashboard are for investigating after it fires.
//
// It only ever OBSERVES. A mismatch is reported, never acted on: halting payments because an
// accounting figure does not tie would turn a reportable condition into an outage, and the circuit
// breaker already exists for when stopping is a deliberate decision.
//
// Returns a stop function; nil means the checker was not started.
func startHubReconciliationChecker(svc *services.HubReconciliationService) func() {
	if svc == nil {
		return nil
	}
	interval := reconciliationInterval()
	if interval <= 0 {
		log.Printf("[hub-reconciliation] disabled by HUB_RECONCILIATION_INTERVAL_SEC=0 — an unattributable Hub balance will not be reported")
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		reconcileOnce(ctx, svc)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				reconcileOnce(ctx, svc)
			}
		}
	}()
	log.Printf("[hub-reconciliation] checker started (every %s)", interval)
	return cancel
}

// reconcileOnce runs one reconciliation and reports the outcome.
func reconcileOnce(ctx context.Context, svc *services.HubReconciliationService) {
	report, err := svc.Reconcile(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return // shutting down, not a finding
		}
		// A failed reconciliation is itself worth saying out loud: it means nothing is watching
		// the balance right now.
		log.Printf("[hub-reconciliation] could not reconcile: %v", err)
		return
	}
	if report.Balanced {
		log.Printf("[hub-reconciliation] balanced — token=%s holder=%s on_chain=%s in_flight=%s",
			report.WToken, report.HolderAddress, report.OnChainBalance, report.ExpectedInFlight)
		return
	}
	// Structured, because this is the line an operator or a log pipeline has to act on.
	payload, mErr := json.Marshal(report)
	if mErr != nil {
		log.Printf("[hub-reconciliation] UNBALANCED — unexplained=%s on_chain=%s in_flight=%s stranded=%s (report not serialisable: %v)",
			report.Unexplained, report.OnChainBalance, report.ExpectedInFlight, report.StrandedTotal, mErr)
		return
	}
	log.Printf("[hub-reconciliation] UNBALANCED — %s of this CB's Hub balance cannot be attributed to a payment: %s",
		report.Unexplained, string(payload))
}

// reconciliationInterval reads HUB_RECONCILIATION_INTERVAL_SEC. Zero disables the checker; an
// invalid value falls back to the default with a warning rather than silently disabling the one
// thing that watches the balance.
func reconciliationInterval() time.Duration {
	raw := os.Getenv("HUB_RECONCILIATION_INTERVAL_SEC")
	if raw == "" {
		return reconciliationDefaultInterval
	}
	secs, err := strconv.Atoi(raw)
	if err != nil || secs < 0 {
		log.Printf("[hub-reconciliation] HUB_RECONCILIATION_INTERVAL_SEC=%q is not a non-negative integer — using %s", raw, reconciliationDefaultInterval)
		return reconciliationDefaultInterval
	}
	return time.Duration(secs) * time.Second
}

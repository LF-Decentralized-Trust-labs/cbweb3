// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/services"
)

// residueRetryDefaultInterval is how often the worker looks for residue returns whose enqueue
// failed. It is deliberately unhurried: the value is not lost, and each attempt talks to the
// issuing CB. Backoff between attempts is the orchestrator's concern; this is only the sweep.
const residueRetryDefaultInterval = 60 * time.Second

// residueRetryBatch bounds one sweep so a large backlog cannot monopolise a tick.
const residueRetryBatch = 50

// startResidueRetryWorker re-drives residue returns that failed to enqueue, until they are in
// flight or exhausted.
//
// Why a worker at all: a failed ENQUEUE creates no bridge position, so the relayer's own queue —
// which retries the legs that did get enqueued — has nothing to pick up. Without this the
// payer's unspent reserve stays on the issuing CB's Hub address indefinitely and the payer
// stays over-debited, with no automatic path back.
//
// Returns a stop function; a nil return means the worker was not started (no orchestrator or
// repository on this gateway, e.g. a read-only deployment).
func startResidueRetryWorker(
	orch *services.CrossCurrencySwapOrchestrator,
	repo services.ResidueRetryRepository,
) func() {
	if orch == nil || repo == nil {
		return nil
	}
	interval := residueRetryInterval()
	if interval <= 0 {
		log.Printf("[residue-retry] disabled by RESIDUE_RETRY_INTERVAL_SEC=0 — a failed residue enqueue will not be retried")
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		// One sweep at start-up: a gateway that crashed mid-swap leaves failures behind, and
		// they should not wait a whole interval.
		orch.RetryFailedResidueReturns(ctx, repo, time.Now(), residueRetryBatch)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				orch.RetryFailedResidueReturns(ctx, repo, time.Now(), residueRetryBatch)
			}
		}
	}()
	log.Printf("[residue-retry] worker started (every %s, up to %d per sweep)", interval, residueRetryBatch)
	return cancel
}

// residueRetryInterval reads RESIDUE_RETRY_INTERVAL_SEC. Zero disables the worker; an invalid
// value falls back to the default with a warning rather than silently disabling recovery.
func residueRetryInterval() time.Duration {
	raw := os.Getenv("RESIDUE_RETRY_INTERVAL_SEC")
	if raw == "" {
		return residueRetryDefaultInterval
	}
	secs, err := strconv.Atoi(raw)
	if err != nil || secs < 0 {
		log.Printf("[residue-retry] RESIDUE_RETRY_INTERVAL_SEC=%q is not a non-negative integer — using %s", raw, residueRetryDefaultInterval)
		return residueRetryDefaultInterval
	}
	return time.Duration(secs) * time.Second
}

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

// bridgeOutRetryDefaultInterval is how often the worker looks for delivery notifications that
// failed. Unhurried on purpose: the backoff between attempts is the orchestrator's concern and
// is measured in minutes, so sweeping faster than this would only find rows that are not due.
const bridgeOutRetryDefaultInterval = 60 * time.Second

// bridgeOutRetryBatch bounds one sweep so a large backlog cannot monopolise a tick.
const bridgeOutRetryBatch = 50

// startBridgeOutRetryWorker re-drives delivery notifications that the beneficiary's central
// bank never accepted, until they are in flight or exhausted.
//
// Why a worker at all: a rejected notification creates no bridge position on CB-B, so that
// CB's relayer queue — which drives the legs that WERE accepted — has nothing to pick up, and
// the Cacti relay forwards exactly once. Without this the swapped value stays on the Hub with
// no beneficiary, the payer stays debited, and the only trace is an error telling the operator
// to contact support. That is the partial settlement the constitution forbids.
//
// Returns a stop function; a nil return means the worker was not started (no orchestrator or
// repository on this gateway, e.g. a read-only deployment).
func startBridgeOutRetryWorker(
	orch *services.CrossCurrencySwapOrchestrator,
	repo services.BridgeOutRetryRepository,
) func() {
	if orch == nil || repo == nil {
		return nil
	}
	interval := bridgeOutRetryInterval()
	if interval <= 0 {
		log.Printf("[bridge-out-retry] disabled by BRIDGE_OUT_RETRY_INTERVAL_SEC=0 — a failed delivery will leave the swapped value on the Hub with no beneficiary and no automatic recovery")
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		// One sweep at start-up: a gateway that crashed mid-swap leaves failures behind, and a
		// stranded delivery should not wait a whole interval.
		orch.RetryFailedBridgeOuts(ctx, repo, time.Now(), bridgeOutRetryBatch)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				orch.RetryFailedBridgeOuts(ctx, repo, time.Now(), bridgeOutRetryBatch)
			}
		}
	}()
	log.Printf("[bridge-out-retry] worker started (every %s, up to %d per sweep)", interval, bridgeOutRetryBatch)
	return cancel
}

// bridgeOutRetryInterval reads BRIDGE_OUT_RETRY_INTERVAL_SEC. Zero disables the worker; an
// invalid value falls back to the default with a warning rather than silently disabling
// recovery — the failure mode this exists to prevent is exactly "nothing tried again".
func bridgeOutRetryInterval() time.Duration {
	raw := os.Getenv("BRIDGE_OUT_RETRY_INTERVAL_SEC")
	if raw == "" {
		return bridgeOutRetryDefaultInterval
	}
	secs, err := strconv.Atoi(raw)
	if err != nil || secs < 0 {
		log.Printf("[bridge-out-retry] BRIDGE_OUT_RETRY_INTERVAL_SEC=%q is not a non-negative integer — using %s", raw, bridgeOutRetryDefaultInterval)
		return bridgeOutRetryDefaultInterval
	}
	return time.Duration(secs) * time.Second
}

// SPDX-License-Identifier: Apache-2.0

// Package services provides the retry path for a cross-currency delivery that was never
// accepted by the beneficiary's central bank.
//
// A cross-currency swap runs in four steps: bridge-in, swap on the Hub, deliver to the
// beneficiary, return the unspent input. Step 3 — the delivery — has two failure points, and
// only one of them was retried:
//
//   - the notification WAS accepted (DELIVERY_NOTIFIED) — CB-B created a bridge position, so
//     its own relayer queue drives the burn and release with backoff and escalates when
//     exhausted;
//   - the notification was REJECTED or never arrived (DELIVERY_FAILED) — no position was
//     created anywhere, the Cacti relay forwards exactly once, and nothing ever tried again.
//     The swapped value stayed on the Hub address that received it, the beneficiary got
//     nothing, and the payer stayed debited.
//
// That second state is the partial settlement the constitution forbids, and the operator was
// told to "contact support". This closes the asymmetry, mirroring the residue return's retry
// (residue_retry.go) deliberately rather than inventing a second scheme: the two failures have
// the same shape and giving them different behaviour would be the surprise.
//
// It is safe to automate because nothing that authorizes the burn is caller-supplied. CB-B
// re-derives the amount, the burn-from address and the recipient from the on-chain swap
// receipt — the relay's own amount_out is cross-checked against LogSwap and its
// swap_sender_address is ignored outright — and the endpoint is idempotent on swap_tx_hash. A
// rejected notification never consumed that hash, so the first successful retry creates the
// position and any later repeat is answered with the existing one, which also repairs a FALSE
// DELIVERY_FAILED where the call succeeded and only the response was lost.
package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

const (
	// bridgeOutMaxAttempts matches residueMaxAttempts so the two retry paths give up at the
	// same point. Beyond it the beneficiary still has nothing and the payer is still debited,
	// which is what DELIVERY_ESCALATED records — a human then decides between a late delivery
	// and a refund, and only a human can, because the choice is not technical.
	bridgeOutMaxAttempts = 5
	// bridgeOutBackoffBaseSeconds is the first delay; each attempt doubles it.
	//
	// The dominant cause of this failure is a beneficiary bank that has not finished
	// onboarding, which is resolved by a person opening a portal — minutes to hours, not
	// milliseconds. So the schedule is deliberately unhurried: 60s doubling gives 60, 120, 240,
	// 480, 960 across the five attempts, covering about 30 minutes in total. Retrying faster
	// would spend the whole budget before the human cause could plausibly clear.
	bridgeOutBackoffBaseSeconds = 60
	// bridgeOutBackoffCapSeconds guards the doubling. Nothing reaches it today —
	// bridgeOutBackoff(bridgeOutMaxAttempts) is 960s and the cap would first bind at attempt 6
	// — and it starts mattering exactly when someone raises the ceiling, which is when it
	// should. A test pins that, so this comment cannot quietly go stale.
	bridgeOutBackoffCapSeconds = 1800
	// bridgeOutMaxDeferralSeconds bounds how long a pair halted by governance may hold a
	// delivery before it needs a human. It must sit WELL above the longest delay the retry loop
	// can schedule (960s today, not the 1800s cap, which nothing reaches) or a routine
	// incident-length pause would escalate every pending delivery instead of waiting it out.
	//
	// A day, matching the residue bound: a pause that outlives a business day has stopped being
	// an incident and become a decision — and a decision to keep a pair closed is not a decision
	// to strand a beneficiary's payment.
	bridgeOutMaxDeferralSeconds = 86400
	// bridgeOutClaimLeaseSeconds hides a claimed row from other sweepers while this one works
	// on it. It only matters when a sweep dies mid-row: every normal outcome writes a real
	// schedule over the lease.
	bridgeOutClaimLeaseSeconds = 300
)

// BridgeOutMaxAttempts exposes the ceiling so the persistence layer can enforce it in the
// query itself, keeping one definition of the bound.
func BridgeOutMaxAttempts() int { return bridgeOutMaxAttempts }

// BridgeOutClaimLease exposes the lease so the repository can write it when claiming.
func BridgeOutClaimLease() time.Duration { return bridgeOutClaimLeaseSeconds * time.Second }

func bridgeOutMaxDeferral() time.Duration { return bridgeOutMaxDeferralSeconds * time.Second }

// bridgeOutBackoff is the delay before the given attempt number, doubling from the base and
// clamped at the cap.
func bridgeOutBackoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	secs := bridgeOutBackoffBaseSeconds
	for i := 1; i < attempts; i++ {
		secs *= 2
		if secs >= bridgeOutBackoffCapSeconds {
			return bridgeOutBackoffCapSeconds * time.Second
		}
	}
	return time.Duration(secs) * time.Second
}

// BridgeOutRetryRepository is the persistence the retry loop needs — a narrow slice of the
// swap repository, so the worker can be tested without one.
type BridgeOutRetryRepository interface {
	// ClaimNextRetryableBridgeOut returns the oldest swap whose delivery notification failed
	// and whose next attempt is due, and CLAIMS it so a concurrent sweeper does not take the
	// same row. Returns (nil, nil) when nothing is due.
	ClaimNextRetryableBridgeOut(ctx context.Context, now time.Time) (*domain.CrossCurrencySwapOperation, error)
	// UpdateBridgeOutDelivery records the outcome of an attempt and, when another is scheduled,
	// when it becomes due. A nil nextAttemptAt means no further attempt.
	//
	// It also CLEARS any deferral window: recording an attempt means the row was actually
	// tried, so a later pause opens a fresh window rather than inheriting an old one and
	// escalating on its first sweep.
	UpdateBridgeOutDelivery(ctx context.Context, swapID string, status domain.BridgeOutDeliveryStatus, attempts int, nextAttemptAt *time.Time) error
	// DeferBridgeOut reschedules a row whose pair is halted WITHOUT touching the attempt
	// counter, recording deferredSince if no window is open yet — the first stamp wins, so the
	// bound measures the whole pause.
	DeferBridgeOut(ctx context.Context, swapID string, nextAttemptAt, deferredSince time.Time) error
	// UpdateBridgeOutPositionID stores the correlation CB-B echoes back once it accepts.
	UpdateBridgeOutPositionID(ctx context.Context, swapID string, positionID string) error
	// MarkDeliveredAfterRetry moves the swap off FAILED and records why, in one write.
	//
	// One write, because the two must not disagree: a reason saying the beneficiary was paid
	// next to a verdict saying the swap failed is exactly the contradiction this replaces. It
	// is a separate method from UpdateFailureReason for the same reason — that one pins the
	// status to FAILED, which is right where it is used and wrong here.
	MarkDeliveredAfterRetry(ctx context.Context, swapID string, reason string) error
}

// RetryFailedBridgeOuts re-drives every delivery notification that failed and is due. Returns
// how many were attempted and how many are now accepted by the beneficiary's central bank.
//
// It reuses the same relay call the first attempt makes, so a retry is the same act rather
// than a parallel implementation that could drift from it.
func (o *CrossCurrencySwapOrchestrator) RetryFailedBridgeOuts(ctx context.Context, repo BridgeOutRetryRepository, now time.Time, limit int) (attempted, recovered int) {
	if repo == nil || o.cactiRelay == nil {
		return 0, 0
	}
	// Claimed one at a time, not listed: with more than one gateway replica sweeping, a plain
	// SELECT hands both the same rows and both dispatch. That does not double-deliver — the
	// endpoint is idempotent on swap_tx_hash — but it burns two attempts against one row's
	// ceiling and doubles the load on the relay for no gain.
	for claimed := 0; claimed < limit; claimed++ {
		// A cancelled context means the process is going away, not that the attempt failed.
		// Counting it would burn attempts, and escalate rows sitting at the ceiling, for no
		// reason other than a restart.
		if ctx.Err() != nil {
			log.Printf("[bridge-out-retry] sweep interrupted after %d attempts: %v", attempted, ctx.Err())
			return attempted, recovered
		}
		// time.Now(), not the sweep's now: the lease is written here and must run from here. A
		// sweep that has already spent minutes on earlier rows would otherwise hand this row a
		// lease already partly in the past.
		op, err := repo.ClaimNextRetryableBridgeOut(ctx, time.Now())
		if err != nil {
			log.Printf("[bridge-out-retry] claim next retryable bridge-out: %v", err)
			return attempted, recovered
		}
		if op == nil {
			return attempted, recovered
		}
		// Governance pausing a pair stops trading on it. Delivering against that same pair
		// while it is paused would leave the one unattended path still moving value, which is
		// what the breaker exists to prevent. Reschedule without consuming an attempt.
		if o.circuitBreakerCheck != nil {
			if halted, cerr := o.circuitBreakerCheck.IsHalted(ctx, op.PoolPair); cerr == nil && halted {
				// Deferring does not consume an attempt, so the ceiling cannot end this wait.
				// Without a bound of its own a pair left paused defers the delivery forever
				// while the beneficiary has nothing and the payer stays debited.
				if op.BridgeOutDeferredSince != nil && now.Sub(*op.BridgeOutDeferredSince) >= bridgeOutMaxDeferral() {
					log.Printf("[bridge-out-retry] swap %s: pool %s has been halted since %s (over %s) — escalating; %s %s stays on the Hub and beneficiary %s has received nothing",
						op.SwapID, op.PoolPair, op.BridgeOutDeferredSince.Format(time.RFC3339), bridgeOutMaxDeferral(),
						op.AmountOut, sanitizeLogValue(op.TargetCurrency), sanitizeLogValue(op.BeneficiaryBankID))
					// The counter passes through unchanged: no attempt was made against this
					// row, and claiming one would misreport why it was given up on.
					o.recordBridgeOutOutcome(ctx, repo, op.SwapID, domain.BridgeOutDeliveryEscalated, op.BridgeOutAttempts, nil)
					continue
				}
				next := now.Add(bridgeOutBackoff(op.BridgeOutAttempts + 1))
				if derr := repo.DeferBridgeOut(ctx, op.SwapID, next, now); derr != nil {
					log.Printf("[bridge-out-retry] swap %s: could not defer while %s is halted: %v", op.SwapID, op.PoolPair, derr)
				}
				log.Printf("[bridge-out-retry] swap %s deferred — pool %s is halted by governance", op.SwapID, op.PoolPair)
				continue
			}
		}
		if o.retryOneBridgeOut(ctx, repo, op, now) {
			recovered++
		}
		attempted++
	}
	if attempted > 0 {
		log.Printf("[bridge-out-retry] attempted=%d recovered=%d", attempted, recovered)
	}
	return attempted, recovered
}

// retryOneBridgeOut re-sends one delivery notification. Reports whether it is now accepted.
func (o *CrossCurrencySwapOrchestrator) retryOneBridgeOut(
	ctx context.Context,
	repo BridgeOutRetryRepository,
	op *domain.CrossCurrencySwapOperation,
	now time.Time,
) bool {
	attempts := op.BridgeOutAttempts + 1

	if op.SwapTxHash == nil || *op.SwapTxHash == "" {
		// Without the hash CB-B cannot verify anything, so no retry can ever succeed. Escalate
		// rather than burn the budget failing the same way five times.
		log.Printf("[bridge-out-retry] swap %s has no swap_tx_hash — cannot be re-delivered; escalating", op.SwapID)
		o.recordBridgeOutOutcome(ctx, repo, op.SwapID, domain.BridgeOutDeliveryEscalated, op.BridgeOutAttempts, nil)
		return false
	}

	// The AMM address is re-resolved rather than stored: it is a property of the pair, and a
	// stored copy would go stale if the pair were ever re-pointed. Best-effort, exactly as on
	// the first attempt — an empty value makes the relay fail safe and refuse the burn.
	ammAddr := ""
	if o.ammAddrResolver != nil {
		if a, aErr := o.ammAddrResolver.AMMAddressFor(ctx, op.PoolPair); aErr == nil {
			ammAddr = a
		} else {
			log.Printf("[bridge-out-retry] swap %s: could not resolve AMM address for pool %s: %v", op.SwapID, op.PoolPair, aErr)
		}
	}

	wrapped := ""
	if o.bridgeAssets != nil {
		wrapped = o.bridgeAssets.WrappedTargetToken
	}

	// Rebuilt from the persisted row, not from a stored payload. Every field here is either a
	// column or re-derived; SwapSenderAddress is deliberately omitted because CB-B ignores it
	// and reads the burn-from address off the receipt instead.
	req := CactiCrossCurrencyBridgeOutRequest{
		CorrelationID:      op.CorrelationID,
		SwapTxHash:         *op.SwapTxHash,
		PoolPair:           op.PoolPair,
		AmountOut:          op.AmountOut,
		BeneficiaryBankID:  op.BeneficiaryBankID,
		SpokeOut:           "spoke-" + strings.ToLower(op.TargetCurrency),
		AmmAddress:         ammAddr,
		WrappedTargetToken: wrapped,
	}

	correlationBack, relayErr := o.cactiRelay.NotifyBridgeOut(ctx, req)
	if relayErr != nil {
		if attempts >= bridgeOutMaxAttempts {
			log.Printf("[bridge-out-retry] swap %s: delivery still failing after %d attempts — escalating; %s %s stays on the Hub and beneficiary %s has received nothing: %v",
				op.SwapID, attempts, op.AmountOut, sanitizeLogValue(op.TargetCurrency), sanitizeLogValue(op.BeneficiaryBankID), relayErr)
			o.recordBridgeOutOutcome(ctx, repo, op.SwapID, domain.BridgeOutDeliveryEscalated, attempts, nil)
			return false
		}
		next := now.Add(bridgeOutBackoff(attempts + 1))
		log.Printf("[bridge-out-retry] swap %s: delivery attempt %d of %d failed, next at %s: %v",
			op.SwapID, attempts, bridgeOutMaxAttempts, next.Format(time.RFC3339), relayErr)
		o.recordBridgeOutOutcome(ctx, repo, op.SwapID, domain.BridgeOutDeliveryFailed, attempts, &next)
		return false
	}

	log.Printf("[bridge-out-retry] swap %s: delivery accepted on attempt %d of %d (echo=%s)", op.SwapID, attempts, bridgeOutMaxAttempts, correlationBack)
	if correlationBack != "" {
		if perr := repo.UpdateBridgeOutPositionID(ctx, op.SwapID, correlationBack); perr != nil {
			log.Printf("[bridge-out-retry] swap %s: could not persist bridge-out position id: %v", op.SwapID, perr)
		}
	}
	// Say what happened, in both fields a person reads. Leaving the original "partial success"
	// text behind would tell an operator the beneficiary was never paid, which stopped being
	// true on this attempt — and leaving the verdict at FAILED said it louder, because that is
	// the column consulted first and the one that decides whether the row is opened at all.
	//
	// The original reason is kept after it: it is why the retry existed, and dropping it would
	// erase the only record that the first delivery was rejected at all.
	//
	// Only this branch promotes the verdict. An attempt that failed again, or one that
	// exhausted the budget, has changed nothing about what the payer was told.
	reason := fmt.Sprintf("delivery recovered on attempt %d of %d — the beneficiary has been paid",
		attempts, bridgeOutMaxAttempts)
	if op.FailureReason != nil && *op.FailureReason != "" {
		reason += " (original failure: " + *op.FailureReason + ")"
	}
	if rerr := repo.MarkDeliveredAfterRetry(ctx, op.SwapID, reason); rerr != nil {
		log.Printf("[bridge-out-retry] swap %s: could not record the recovery on the swap record: %v", op.SwapID, rerr)
	}
	o.recordBridgeOutOutcome(ctx, repo, op.SwapID, domain.BridgeOutDeliveryNotified, attempts, nil)
	return true
}

// recordBridgeOutOutcome persists one attempt's result. It never returns an error: the sweep
// must continue to the next row, and a write that fails is logged so the row is visible rather
// than silently stuck.
func (o *CrossCurrencySwapOrchestrator) recordBridgeOutOutcome(
	ctx context.Context,
	repo BridgeOutRetryRepository,
	swapID string,
	status domain.BridgeOutDeliveryStatus,
	attempts int,
	nextAttemptAt *time.Time,
) {
	if err := repo.UpdateBridgeOutDelivery(ctx, swapID, status, attempts, nextAttemptAt); err != nil {
		log.Printf("[bridge-out-retry] swap %s: could not record delivery outcome %s: %v", swapID, status, err)
	}
}

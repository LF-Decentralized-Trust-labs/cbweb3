// SPDX-License-Identifier: Apache-2.0

// Package services provides the retry path for a residue return whose enqueue failed.
//
// A cross-currency swap bridges the worst-case input (MaxAmountIn) because the real cost is
// only known after the AMM trade. Step 4 gives the unspent part back. That return has two
// failure points, and only one of them was retried:
//
//   - the leg WAS enqueued (RETURN_ENQUEUED) — a bridge position exists, so the relayer's own
//     queue retries it with backoff and escalates to RECONCILIATION_REQUIRED when exhausted;
//   - the enqueue itself failed (RETURN_FAILED) — no position was created, so the relayer queue
//     has nothing to pick up and nothing ever tried again. The bank's unspent reserve stayed on
//     the issuing CB's Hub address indefinitely, and the bank could not even see it.
//
// This closes that asymmetry. It is safe to automate because nothing about the request is
// caller-supplied: the issuing CB derives the amount from the bridge-in position it created
// plus the on-chain LogSwap, and the endpoint is idempotent on (swap_tx_hash, RESIDUE). A
// retry that turns out to be a duplicate is answered with the existing position — which also
// repairs a FALSE RETURN_FAILED, where the enqueue actually succeeded and only the response
// was lost.
package services

import (
	"context"
	"log"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/api-gateway/internal/domain"
)

const (
	// residueMaxAttempts mirrors the relayer worker's ceiling so both retry paths give up at
	// the same point. Beyond it the value still sits on the CB's Hub address and needs a
	// human, which is what RETURN_ESCALATED records.
	residueMaxAttempts = 5
	// residueBackoffCapSeconds caps the exponential backoff. It has to sit WELL above the
	// sweep interval (60s by default) or the schedule never delays anything and the whole
	// budget is spent in one attempt per tick: a CB restart of a few minutes would then
	// escalate every pending residue, which is supposed to mean "a human is needed", not "the
	// CB was redeployed".
	residueBackoffCapSeconds = 1800
	// residueBackoffBaseSeconds is the first delay; each attempt doubles it up to the cap.
	residueBackoffBaseSeconds = 60
	// residueClaimLeaseSeconds is how long a claimed row is hidden from other sweepers while
	// this one works on it. It only matters when a sweep dies mid-row: every normal outcome —
	// success, failure, or a breaker deferral — writes a real schedule over the lease. So it
	// wants to be long enough to cover a dispatch to the issuing CB and short enough that a
	// crashed sweep does not park a residue for an hour.
	residueClaimLeaseSeconds = 300
)

// ResidueMaxAttempts exposes the attempt ceiling so the persistence layer can enforce it in the
// query itself, keeping one definition of the bound.
func ResidueMaxAttempts() int { return residueMaxAttempts }

// ResidueClaimLease exposes the claim lease for the same reason: the persistence layer writes
// it, this package owns what it means.
func ResidueClaimLease() time.Duration { return residueClaimLeaseSeconds * time.Second }

// ResidueRetryRepository is the persistence the retry loop needs. It is a narrow slice of the
// swap repository so the worker can be tested without one.
type ResidueRetryRepository interface {
	// ClaimRetryableResidues returns swaps whose residue return failed to enqueue and whose
	// next attempt is due, oldest first, bounded by limit — and CLAIMS them, so a concurrent
	// sweeper does not pick up the same rows.
	ClaimRetryableResidues(ctx context.Context, now time.Time, limit int) ([]domain.CrossCurrencySwapOperation, error)
	// UpdateResidue records the outcome of an attempt (status, amount, position).
	UpdateResidue(ctx context.Context, swapID, amount, positionID string, status domain.ResidueReturnStatus) error
	// RecordResidueAttempt persists the attempt counter and when the next one becomes due.
	// A nil nextAttemptAt means no further attempt is scheduled.
	RecordResidueAttempt(ctx context.Context, swapID string, attempts int, nextAttemptAt *time.Time) error
}

// RetryFailedResidueReturns re-drives every residue return that failed to enqueue and is due.
// Returns how many were attempted and how many now have a return in flight.
//
// It reuses dispatchResidueReturn, so the sovereign path (delegate to the issuing CB) and the
// local path (this gateway IS the issuing CB) behave exactly as they do on the first attempt —
// the retry is the same act, not a parallel implementation that could drift from it.
func (o *CrossCurrencySwapOrchestrator) RetryFailedResidueReturns(ctx context.Context, repo ResidueRetryRepository, now time.Time, limit int) (attempted, recovered int) {
	if repo == nil {
		return 0, 0
	}
	// Claimed, not merely listed: with more than one gateway replica sweeping, a plain SELECT
	// hands both the same rows and both dispatch. That does not double-refund — the endpoint is
	// idempotent on (swap_tx_hash, RESIDUE) — but it burns two attempts against one row's
	// ceiling and doubles the load on the issuing CB for no gain.
	pending, err := repo.ClaimRetryableResidues(ctx, now, limit)
	if err != nil {
		log.Printf("[residue-retry] claim retryable residues: %v", err)
		return 0, 0
	}
	for i := range pending {
		// A cancelled context means the process is going away, not that the attempt failed.
		// Counting it would burn attempts — and escalate rows sitting at the ceiling — for no
		// reason other than a restart.
		if ctx.Err() != nil {
			log.Printf("[residue-retry] sweep interrupted after %d attempts: %v", attempted, ctx.Err())
			return attempted, recovered
		}
		op := &pending[i]
		// Governance pausing a pair stops trading on it; deriving refunds from that same pair's
		// LogSwap events while it is paused would leave the one unattended path still moving
		// value. Reschedule without consuming an attempt — the residue is not lost by waiting.
		if o.circuitBreakerCheck != nil {
			if halted, cerr := o.circuitBreakerCheck.IsHalted(ctx, op.PoolPair); cerr == nil && halted {
				next := now.Add(residueBackoff(op.ResidueAttempts + 1))
				if rerr := repo.RecordResidueAttempt(ctx, op.SwapID, op.ResidueAttempts, &next); rerr != nil {
					log.Printf("[residue-retry] swap %s: could not defer while %s is halted: %v", op.SwapID, op.PoolPair, rerr)
				}
				log.Printf("[residue-retry] swap %s deferred — pool %s is halted by governance", op.SwapID, op.PoolPair)
				continue
			}
		}
		if o.retryOne(ctx, repo, op, now) {
			recovered++
		}
		attempted++
	}
	if attempted > 0 {
		log.Printf("[residue-retry] attempted=%d recovered=%d", attempted, recovered)
	}
	return attempted, recovered
}

// retryOne re-drives a single swap's residue return. Reports whether the return is now in
// flight.
func (o *CrossCurrencySwapOrchestrator) retryOne(
	ctx context.Context,
	repo ResidueRetryRepository,
	op *domain.CrossCurrencySwapOperation,
	now time.Time,
) bool {
	residue, ok := new(big.Int).SetString(op.ResidueAmount, 10)
	if !ok || residue.Sign() <= 0 {
		// Nothing to return, or an unparseable amount that a retry cannot fix. Escalate so it
		// is visible rather than retried forever against a value that will never make sense.
		log.Printf("[residue-retry] swap %s has an unusable residue_amount %q — escalating", op.SwapID, op.ResidueAmount)
		o.recordResidueOutcome(ctx, repo, op.SwapID, op.ResidueAmount, "", domain.ResidueReturnEscalated, op.ResidueAttempts+1, nil)
		return false
	}

	bridgeInPositionID := ""
	if op.BridgeInPositionID != nil {
		bridgeInPositionID = *op.BridgeInPositionID
	}
	swapTxHash := ""
	if op.SwapTxHash != nil {
		swapTxHash = *op.SwapTxHash
	}
	// Both are the issuing CB's only way to derive the amount. Without them the request cannot
	// be authorised on the CB side, and no number of retries changes that.
	if bridgeInPositionID == "" || swapTxHash == "" {
		log.Printf("[residue-retry] swap %s lacks bridge_in_position_id/swap_tx_hash — escalating (the issuing CB cannot derive the amount)", op.SwapID)
		o.recordResidueOutcome(ctx, repo, op.SwapID, op.ResidueAmount, "", domain.ResidueReturnEscalated, op.ResidueAttempts+1, nil)
		return false
	}

	req := CrossCurrencySwapRequest{
		SwapID:         op.SwapID,
		CorrelationID:  op.CorrelationID,
		SourceCurrency: op.SourceCurrency,
		TargetCurrency: op.TargetCurrency,
		PoolPair:       op.PoolPair,
		AmountOut:      op.AmountOut,
		MaxAmountIn:    op.MaxAmountIn,
		PayerBankID:    op.PayerBankID,
	}
	spokeIn := "spoke-" + lowerASCII(op.SourceCurrency)

	// The Hub address holding the unspent input. The first attempt may have used the executing
	// CB's address (swapResult.HubSenderAddress), which is not persisted, so this substitutes
	// this gateway's signer.
	//
	// That is correct only while a delegated Step 2 implies a delegated Step 4: the hub-swap
	// relay and the residue relay are wired together in the same CENTRAL_BANK_API_URL block, so
	// residueRelay == nil (the only path that reads this address) implies hubSwapRelay == nil
	// and the first attempt used this same signer. On the sovereign path the issuing CB ignores
	// what we pass and derives the sender from the on-chain LogSwap. Decoupling those two
	// wirings would break this: persist the address on the swap record before doing so.
	positionID, err := o.dispatchResidueReturn(ctx, req, residue, bridgeInPositionID, swapTxHash, spokeIn, o.hubSignerAddress)
	attempts := op.ResidueAttempts + 1
	if err == nil && positionID == "" {
		// A success carrying no position is not a return in flight. The issuing CB answers this
		// way when its own derivation yields nothing to give back ("no_residue"), which
		// contradicts the amount recorded here. Recording RETURN_ENQUEUED would close the case
		// on a disagreement nobody ever sees; escalate it instead.
		log.Printf("[residue-retry] swap %s: the issuing CB reported nothing to return, but %s %s is recorded here — escalating for reconciliation",
			op.SwapID, op.ResidueAmount, sanitizeCurrency(op.SourceCurrency))
		o.recordResidueOutcome(ctx, repo, op.SwapID, op.ResidueAmount, "", domain.ResidueReturnEscalated, attempts, nil)
		return false
	}
	if err == nil {
		o.recordResidueOutcome(ctx, repo, op.SwapID, op.ResidueAmount, positionID, domain.ResidueReturnEnqueued, attempts, nil)
		log.Printf("[residue-retry] swap %s: residue return enqueued on attempt %d (position=%s amount=%s)",
			op.SwapID, attempts, positionID, op.ResidueAmount)
		return true
	}

	if attempts >= residueMaxAttempts {
		log.Printf("[residue-retry] swap %s exhausted after %d attempts: %v — %s %s stays on the issuing CB's Hub address and the payer %s remains over-debited until reconciled",
			op.SwapID, attempts, err, op.ResidueAmount, sanitizeCurrency(op.SourceCurrency), sanitizeLogValue(op.PayerBankID))
		o.recordResidueOutcome(ctx, repo, op.SwapID, op.ResidueAmount, "", domain.ResidueReturnEscalated, attempts, nil)
		return false
	}

	next := now.Add(residueBackoff(attempts))
	log.Printf("[residue-retry] swap %s attempt %d failed (%v) — next attempt at %s",
		op.SwapID, attempts, err, next.Format(time.RFC3339))
	if rerr := repo.RecordResidueAttempt(ctx, op.SwapID, attempts, &next); rerr != nil {
		// The attempt counter is what bounds this loop. Losing the write silently means the row
		// is due again immediately with the same count, so the issuing CB is re-hit every sweep
		// and the ceiling is never reached.
		log.Printf("[residue-retry] swap %s: could not record attempt %d (%v) — the row stays due and the attempt budget did not advance",
			op.SwapID, attempts, rerr)
	}
	return false
}

// recordResidueOutcome persists a terminal (or corrective) outcome, surfacing either write
// failure. Both writes matter: the status is what takes the row out of the retryable set, and
// the counter is what bounds the loop, so a silent failure here re-dispatches a real refund on
// every sweep.
func (o *CrossCurrencySwapOrchestrator) recordResidueOutcome(
	ctx context.Context,
	repo ResidueRetryRepository,
	swapID, amount, positionID string,
	status domain.ResidueReturnStatus,
	attempts int,
	nextAttemptAt *time.Time,
) {
	if err := repo.UpdateResidue(ctx, swapID, amount, positionID, status); err != nil {
		log.Printf("[residue-retry] swap %s: could not record status %s (%v) — the row stays retryable and will be re-dispatched",
			swapID, status, err)
	}
	if err := repo.RecordResidueAttempt(ctx, swapID, attempts, nextAttemptAt); err != nil {
		log.Printf("[residue-retry] swap %s: could not record attempt %d (%v)", swapID, attempts, err)
	}
}

// sanitizeCurrency and sanitizeLogValue strip CR/LF from values that reach a log line. The
// currency arrives from the request body with no format validation, so an injected newline
// could forge a log entry — in this very trail, the one built for reconciliation.
func sanitizeCurrency(s string) string { return sanitizeLogValue(s) }

func sanitizeLogValue(s string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(s)
}

// residueBackoff is exponential in the attempt count, capped, matching the relayer worker's
// shape so the two retry paths behave alike under a shared outage.
func residueBackoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	secs := math.Min(residueBackoffBaseSeconds*math.Pow(2, float64(attempts-1)), float64(residueBackoffCapSeconds))
	return time.Duration(secs) * time.Second
}

// lowerASCII lowercases an ASCII currency code without pulling in strings for one call site.
func lowerASCII(s string) string {
	out := []byte(s)
	for i, c := range out {
		if c >= 'A' && c <= 'Z' {
			out[i] = c + ('a' - 'A')
		}
	}
	return string(out)
}

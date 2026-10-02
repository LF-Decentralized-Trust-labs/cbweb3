// SPDX-License-Identifier: Apache-2.0

// Whether the bridge's Execute button may be pressed, and what the operator is told.
//
// An expired quote used to disable the button outright, with the reason carried only in a
// `title` attribute — a hover tooltip, which is invisible on touch, unreliable with a
// screen reader, and not where anyone looks when a button stops responding.
//
// That gating was also unnecessary. The store already recovers: `executeSwap` refreshes an
// expired quote before submitting and retries once on a QUOTE_EXPIRED response, then sets
// `quoteWasRefreshed` so the page can say the price moved. Blocking the button is what kept
// the operator away from a recovery that already existed.
//
// So expiry stops being a gate and becomes a visible notice. The gates that remain are the
// ones a refresh cannot fix: no pool, no quote at all, empty fields, or a submission already
// in flight.

export type ExecuteGatingInput = {
  /** The store's step; anything other than "idle" means a submission is already running. */
  step: string;
  /** Whether the selected pool is ACTIVE on the hub. */
  isPoolActive: boolean;
  /** False when no quote has been fetched yet. */
  hasQuote: boolean;
  /** Whether the quote's TTL has elapsed. Not a gate — it drives the notice. */
  isQuoteExpired: boolean;
  amountOut: string;
  amountIn: string;
  beneficiaryBankId: string;
};

export type ExecuteGating = {
  disabled: boolean;
  /** Visible text to render beside the button, or null when there is nothing to say. */
  notice: string | null;
};

export function evaluateExecuteGating(input: ExecuteGatingInput): ExecuteGating {
  const disabled =
    input.step !== "idle"
    || !input.isPoolActive
    || !input.hasQuote
    || !input.amountOut.trim()
    || !input.amountIn.trim()
    || !input.beneficiaryBankId.trim();

  // Only worth saying once there is a quote to have expired, and not while a submission is
  // already running — the operator is watching progress then, not the price.
  const notice =
    input.hasQuote && input.isQuoteExpired && input.step === "idle"
      ? "This quote has expired. Executing will price it again at the current rate before submitting."
      : null;

  return { disabled, notice };
}

// SPDX-License-Identifier: Apache-2.0

/**
 * One explanation of a failed API request, shared by every Scenario A portal.
 *
 * The sibling of `login-errors.ts`, for the other 31 call sites. That module fixed the sign-in
 * routes: the gateway grew stable `code` fields and the five portals stopped rendering axios's own
 * message. Everything else in the bank portal kept doing exactly what sign-in used to — every store
 * and page surfaced `error.message`, so an operator read "Request failed with status code 502"
 * whichever of two hundred handlers had refused, and whatever it had said about why.
 *
 * The case that found it: a divergent INTERNAL_RELAY_AUTH_SECRET makes the central bank answer 401;
 * the bank's gateway wraps that into a 502 whose body names the cause; the store kept
 * `error.message` and discarded `response.data`. Neither the operator nor support could tell a bad
 * relay credential from a central bank that was down. Measured, with the chain written out, in
 * docs/guard-parity.md.
 *
 * Why this is a separate function rather than an extension of loginErrorMessage:
 *
 *   - loginErrorMessage maps a CODE to curated copy, because sign-in has five outcomes and the
 *     words for each are a product decision that must agree across five portals.
 *   - These routes carry no codes. Two hundred and fourteen handlers answer with prose in `error`
 *     and nothing else. There is no mapping to make — the job is to stop throwing that prose away.
 *
 * Merging them would mean either inventing copy for two hundred handlers or letting the login
 * mapping fall through to raw prose, and the second is how curated copy quietly stops being used.
 */

/**
 * The copy this module owns, in one place.
 *
 * Only two strings: everything else on this path is the gateway's own prose, deliberately. No i18n
 * in these apps, so nothing is localised; keeping the strings named gives a future translation pass
 * one seam instead of thirty-one catch blocks.
 */
export const REQUEST_ERROR_COPY = {
  unreachable: "Could not reach the gateway. Check your connection and try again.",
  /**
   * Keeps the status visible without pretending to know the cause. The caller's own fallback says
   * WHAT failed ("Unable to load the statement"); the status is the one fact support can act on,
   * and dropping it is how "something went wrong" screens happen.
   */
  withStatus: (fallback: string, status: number) => `${fallback} (HTTP ${status}).`,
} as const;

/**
 * The parts of a rejected request this reads.
 *
 * Duck-typed rather than taking an AxiosError, so this package does not depend on the HTTP client
 * the apps happen to use. Same shape as login-errors.ts, for the same reason.
 */
type HttpFailure = {
  isAxiosError?: unknown;
  response?: { status?: unknown; data?: unknown };
};

const asHttpFailure = (error: unknown): HttpFailure | null =>
  typeof error === "object" && error !== null ? (error as HttpFailure) : null;

/** A string is prose only if it says something; blank prose renders an empty error box. */
const prose = (value: unknown): string | null => {
  if (typeof value !== "string") {
    return null;
  }
  const trimmed = value.trim();
  return trimmed === "" ? null : trimmed;
};

/**
 * explanationFrom pulls the gateway's own words out of a response body.
 *
 * `error` first and `message` second because that is the split in this gateway: 214 handlers answer
 * with `error`, two with `message`. Reading only the first would leave those two rendering the
 * fallback, which looks exactly like the defect never got fixed.
 */
const explanationFrom = (data: unknown): string | null => {
  const bare = prose(data);
  if (bare !== null) {
    return bare;
  }
  if (typeof data !== "object" || data === null) {
    return null;
  }
  const body = data as { error?: unknown; message?: unknown };
  return prose(body.error) ?? prose(body.message);
};

/**
 * apiErrorMessage turns whatever a request threw into something an operator can act on.
 *
 * `fallback` says what the caller was doing ("Unable to load the statement") and is used when the
 * failure explains nothing itself. It is required rather than defaulted: a generic default would be
 * silently correct at every call site and useful at none.
 *
 * On the gateway's prose reaching the browser verbatim: it already does. The body is sent over the
 * wire whether or not anything renders it, so displaying it exposes nothing new — it only stops the
 * operator from being the one person who cannot see it. If a particular message is judged too
 * internal to show, the fix belongs in the handler that writes it, not in a portal that hides it.
 */
export function apiErrorMessage(error: unknown, fallback: string): string {
  const failure = asHttpFailure(error);

  if (failure?.response) {
    const explanation = explanationFrom(failure.response.data);
    if (explanation !== null) {
      return explanation;
    }
    const status = failure.response.status;
    if (typeof status === "number") {
      return REQUEST_ERROR_COPY.withStatus(fallback, status);
    }
    return fallback;
  }

  // No response at all. Only the HTTP client's own error means the gateway was unreachable; an
  // Error the app threw carries its own explanation and must not be replaced by a network story.
  if (failure?.isAxiosError === true) {
    return REQUEST_ERROR_COPY.unreachable;
  }
  if (error instanceof Error && error.message !== "") {
    return error.message;
  }
  return fallback;
}

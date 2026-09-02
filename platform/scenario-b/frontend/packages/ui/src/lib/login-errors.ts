// SPDX-License-Identifier: Apache-2.0

/**
 * One explanation of a failed sign-in, shared by every Scenario B portal.
 *
 * All five portals used to render axios's own `error.message`, so an operator saw "Request failed
 * with status code 400" and could not tell a wrong secret from an empty field from a gateway that
 * was down. The gateway already wrote a usable message; nothing read it.
 *
 * The mapping keys on the `code` the gateway now returns, not on the prose (which would break on any
 * rewording) and not on the status alone (400 covers both a missing field and a malformed body). It
 * lives here rather than in each app because the copy has to agree across the five: a message that
 * drifts per portal is how an operator learns to distrust all of them.
 */

/**
 * The copy, in one place.
 *
 * There is no i18n in these apps today, so this is not localised — but keeping every string behind
 * one exported object means a future translation pass has a single seam to work on instead of five
 * `catch` blocks. Naming it is the whole point; do not inline these strings at call sites.
 */
export const LOGIN_ERROR_COPY = {
  invalidCredentials:
    "Incorrect client ID or secret. Check both and try again.",
  missingCredentials: "Enter both the client ID and the client secret.",
  invalidRequest:
    "The portal sent a request the gateway could not read. Reload the page and try again.",
  serviceUnavailable:
    "The authentication service is not answering. Wait a moment and try again, or contact your operator.",
  sessionExpired: "Your session has ended. Sign in again.",
  unreachable:
    "Could not reach the gateway. Check your connection and try again.",
  /** Keeps the status visible for support without pretending to know the cause. */
  unexpected: (status: number) =>
    `Sign-in failed (HTTP ${status}). Contact your operator if it continues.`,
  fallback: "Could not sign in. Try again, or contact your operator.",
} as const;

/** The codes the gateway's auth routes return. Mirrors the Go constants in handlers/auth.go. */
const CODE_TO_MESSAGE: Readonly<Record<string, string>> = {
  INVALID_CREDENTIALS: LOGIN_ERROR_COPY.invalidCredentials,
  MISSING_CREDENTIALS: LOGIN_ERROR_COPY.missingCredentials,
  INVALID_REQUEST: LOGIN_ERROR_COPY.invalidRequest,
  AUTH_SERVICE_UNAVAILABLE: LOGIN_ERROR_COPY.serviceUnavailable,
  // Neither of these is a rejected credential. A restore probe on a page with no session answers
  // MISSING_REFRESH_TOKEN, and rendering that as a login failure is what told an operator their
  // credentials were wrong when nothing had been submitted.
  MISSING_REFRESH_TOKEN: LOGIN_ERROR_COPY.sessionExpired,
  INVALID_REFRESH_TOKEN: LOGIN_ERROR_COPY.sessionExpired,
};

/**
 * The parts of a rejected request this mapping reads.
 *
 * Duck-typed rather than taking an AxiosError, so this package does not depend on the HTTP client
 * the apps happen to use. The two fields below are all the decision needs.
 */
type HttpFailure = {
  isAxiosError?: unknown;
  response?: { status?: unknown; data?: unknown };
};

const asHttpFailure = (error: unknown): HttpFailure | null =>
  typeof error === "object" && error !== null ? (error as HttpFailure) : null;

const codeOf = (failure: HttpFailure): string | null => {
  const data = failure.response?.data;
  if (typeof data !== "object" || data === null) {
    return null;
  }
  const code = (data as { code?: unknown }).code;
  return typeof code === "string" && code !== "" ? code : null;
};

/**
 * loginErrorMessage turns whatever a sign-in attempt threw into something an operator can act on.
 *
 * An error the app raised itself is returned as written: those messages are deliberate (a portal
 * refusing a PKI flow it does not implement, for one) and this mapping has nothing better to say
 * about them.
 */
export function loginErrorMessage(error: unknown): string {
  const failure = asHttpFailure(error);

  if (failure?.response) {
    const code = codeOf(failure);
    if (code && code in CODE_TO_MESSAGE) {
      return CODE_TO_MESSAGE[code] as string;
    }
    const status = failure.response.status;
    // A response with no code we know: 401 is still a refusal, whatever the body says.
    if (status === 401) {
      return LOGIN_ERROR_COPY.invalidCredentials;
    }
    if (typeof status === "number") {
      return LOGIN_ERROR_COPY.unexpected(status);
    }
    return LOGIN_ERROR_COPY.fallback;
  }

  // No response at all. Only an HTTP client's own error means the gateway was unreachable; an Error
  // the app threw carries its own explanation and must not be replaced by a network story.
  if (failure?.isAxiosError === true) {
    return LOGIN_ERROR_COPY.unreachable;
  }
  if (error instanceof Error && error.message !== "") {
    return error.message;
  }
  return LOGIN_ERROR_COPY.fallback;
}

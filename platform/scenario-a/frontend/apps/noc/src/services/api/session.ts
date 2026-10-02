// SPDX-License-Identifier: Apache-2.0

// Session lifecycle for the NOC portal.
//
// This replaced token.ts, which kept the access and refresh tokens in localStorage and ran
// the Keycloak grant from the browser. Any script on the page could read them there — the
// exposure this change closes. The session now lives in an HttpOnly cookie the browser
// attaches on its own and JavaScript cannot see, which is why nothing here holds a token:
// there is no token to hold.
//
// What survives from the old module is the part that was never about storage: renewing
// before the access token expires, once, no matter how many pages ask at the same time. The
// portal sits on a videowall and polls several views, so a naive design fires a burst of
// refreshes the moment the token ages out.

import axios from "axios";

/** Backend base URL. The same origin the http client talks to. */
const baseURL = import.meta.env.VITE_NOC_BACKEND_URL ?? "/api/v1";

// A bare axios instance, not the shared httpClient: this is called FROM that client's
// interceptor, and reusing it would recurse.
const authClient = axios.create({ baseURL, withCredentials: true, withXSRFToken: true });

// Session-expired notification. The auth store registers a handler so ProtectedRoute can
// send the operator to /login once renewal is no longer possible.
let sessionExpiredHandler: (() => void) | null = null;

export function setSessionExpiredHandler(handler: () => void): void {
  sessionExpiredHandler = handler;
}

export function notifySessionExpired(): void {
  sessionExpiredHandler?.();
}

// Single-flight: the portal polls several pages concurrently, so an expiring session would
// otherwise fire several refreshes at once — and each one rotates the refresh token at the
// realm, so the losers of the race would be renewing with a token that has just been
// replaced.
let inFlight: Promise<boolean> | null = null;

/**
 * Renews the session through the backend, which rewrites the cookies.
 *
 * Returns whether a session is still available. There is deliberately no token in the
 * return: the caller cannot use one, and handing it back would be the beginning of putting
 * it somewhere readable again.
 */
export function refreshSession(): Promise<boolean> {
  if (inFlight) return inFlight;
  inFlight = performRefresh().finally(() => {
    inFlight = null;
  });
  return inFlight;
}

async function performRefresh(): Promise<boolean> {
  try {
    await authClient.post("/auth/refresh", {});
    return true;
  } catch (error) {
    // A 401 means the refresh cookie is gone or the realm rejected it: the session is over.
    // Anything else is a transient failure — a network blip, the backend restarting — and
    // reporting it as an expiry would log an operator out mid-incident over a hiccup.
    const status = (error as { response?: { status?: number } })?.response?.status;
    return status !== undefined && status !== 401;
  }
}

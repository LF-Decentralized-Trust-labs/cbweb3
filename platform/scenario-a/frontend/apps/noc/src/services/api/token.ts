// SPDX-License-Identifier: Apache-2.0

// Token lifecycle for the NOC portal: storage, expiry inspection and the Keycloak
// refresh-token grant. The access token lives 5 minutes (realm accessTokenLifespan),
// so a portal left open on a NOC videowall must renew it or the operator is thrown
// back to the login screen mid-incident. Kept out of http-client.ts so the expiry
// math stays unit-testable without axios or a browser.

const ACCESS_KEY = "noc_access_token";
const REFRESH_KEY = "noc_refresh_token";

export const keycloakConfig = {
  url: import.meta.env.VITE_KEYCLOAK_URL ?? "http://localhost:8080",
  realm: import.meta.env.VITE_KEYCLOAK_REALM ?? "cbweb3",
  clientId: import.meta.env.VITE_KEYCLOAK_CLIENT_ID ?? "cbweb3-noc",
};

/** Seconds before `exp` at which a token is already treated as due for renewal. */
export const REFRESH_SKEW_SECONDS = 60;

export function readAccessToken(): string | null {
  return localStorage.getItem(ACCESS_KEY);
}

export function readRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_KEY);
}

export function storeTokens(accessToken: string, refreshToken: string): void {
  localStorage.setItem(ACCESS_KEY, accessToken);
  localStorage.setItem(REFRESH_KEY, refreshToken);
}

export function clearTokens(): void {
  localStorage.removeItem(ACCESS_KEY);
  localStorage.removeItem(REFRESH_KEY);
}

/** Decodes a JWT payload without verifying the signature (display/expiry use only). */
export function decodeJwtPayload(token: string): Record<string, unknown> {
  try {
    const segment = token.split(".")[1];
    if (!segment) return {};
    const base64 = segment.replace(/-/g, "+").replace(/_/g, "/");
    return JSON.parse(atob(base64)) as Record<string, unknown>;
  } catch {
    return {};
  }
}

/** `exp` claim in seconds, or null when absent/unparseable. */
export function decodeJwtExp(token: string): number | null {
  const exp = decodeJwtPayload(token)["exp"];
  return typeof exp === "number" ? exp : null;
}

/**
 * Whether a token should be renewed now. A token with no readable `exp` is treated
 * as due, so a malformed token is refreshed rather than used until it 401s.
 */
export function isDueForRefresh(exp: number | null, nowMs: number, skewSeconds = REFRESH_SKEW_SECONDS): boolean {
  if (exp === null) return true;
  return nowMs / 1000 >= exp - skewSeconds;
}

// Session-expired notification. The auth store registers a handler so ProtectedRoute
// can send the operator to /login once renewal is no longer possible.
let sessionExpiredHandler: (() => void) | null = null;

export function setSessionExpiredHandler(handler: () => void): void {
  sessionExpiredHandler = handler;
}

export function notifySessionExpired(): void {
  clearTokens();
  sessionExpiredHandler?.();
}

// Single-flight: the portal polls three pages concurrently, so an expiring token
// would otherwise fire several refreshes at once and race on localStorage.
let inFlight: Promise<string | null> | null = null;

/** Runs the refresh-token grant, returning the new access token or null on failure. */
export function refreshAccessToken(): Promise<string | null> {
  if (inFlight) return inFlight;
  inFlight = performRefresh().finally(() => {
    inFlight = null;
  });
  return inFlight;
}

async function performRefresh(): Promise<string | null> {
  const refreshToken = readRefreshToken();
  if (!refreshToken) return null;

  const body = new URLSearchParams({
    grant_type: "refresh_token",
    client_id: keycloakConfig.clientId,
    refresh_token: refreshToken,
  });

  try {
    const res = await fetch(
      `${keycloakConfig.url}/realms/${keycloakConfig.realm}/protocol/openid-connect/token`,
      { method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" }, body },
    );
    if (!res.ok) {
      // Refresh token expired (SSO idle/max lifespan) or revoked — session is over.
      return null;
    }
    const tokens = (await res.json()) as { access_token: string; refresh_token: string };
    storeTokens(tokens.access_token, tokens.refresh_token);
    return tokens.access_token;
  } catch {
    // Network failure: keep the existing tokens so a transient outage does not log out.
    return null;
  }
}

/**
 * Returns a usable access token, renewing it first when it is within the skew window.
 * Null means there is no session to work with.
 */
export async function ensureFreshToken(): Promise<string | null> {
  const token = readAccessToken();
  if (!token) {
    return refreshAccessToken();
  }
  const exp = decodeJwtExp(token);
  if (isDueForRefresh(exp, Date.now())) {
    const refreshed = await refreshAccessToken();
    if (refreshed) return refreshed;
    // Renewal failed (e.g. transient network error): keep using the current token
    // while it has not actually expired — zero skew means "expired for real".
    return isDueForRefresh(exp, Date.now(), 0) ? null : token;
  }
  return token;
}

// SPDX-License-Identifier: Apache-2.0

import { authApi } from "./auth.api";

// Proactive silent token refresh.
//
// The access token is short-lived (Keycloak default ~5 min). Rather than letting
// it expire and relying on the reactive 401 interceptor to recover (a small blip
// on the request that happens to race the expiry), we renew it a little BEFORE it
// expires so an active session never trips a 401.
//
// The scheduler re-arms itself from each refreshed token's lifespan, so a single
// call to scheduleTokenRefresh() (on login / session restore) keeps the session
// alive for as long as the refresh token / SSO session allows. It is deliberately
// additive: a proactive refresh failure only clears the timer — it NEVER logs the
// user out. The reactive 401 interceptor remains the single authority on when a
// session has truly ended.
const REFRESH_SKEW_SECONDS = 30; // renew this long before the token expires
const MIN_DELAY_SECONDS = 20; // floor so a short lifespan can't busy-loop

let timer: ReturnType<typeof setTimeout> | null = null;

export function scheduleTokenRefresh(expiresInSeconds: number): void {
  cancelTokenRefresh();
  if (!Number.isFinite(expiresInSeconds) || expiresInSeconds <= 0) {
    return;
  }
  const delayMs = Math.max(expiresInSeconds - REFRESH_SKEW_SECONDS, MIN_DELAY_SECONDS) * 1000;
  timer = setTimeout(() => {
    void authApi
      .refresh()
      .then((result) => scheduleTokenRefresh(result.expiresIn))
      .catch(() => cancelTokenRefresh());
  }, delayMs);
}

export function cancelTokenRefresh(): void {
  if (timer !== null) {
    clearTimeout(timer);
    timer = null;
  }
}

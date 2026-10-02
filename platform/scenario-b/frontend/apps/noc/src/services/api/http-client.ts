// SPDX-License-Identifier: Apache-2.0

import axios, { type InternalAxiosRequestConfig } from "axios";
import { useFreshnessStore } from "../../stores/freshness.store";
import { notifySessionExpired, refreshSession } from "./session";

export const httpClient = axios.create({
  baseURL: import.meta.env.VITE_NOC_BACKEND_URL ?? "/api/v1",
  // The session is a cookie now, so the browser has to be told to send it. Without this
  // every request after login is anonymous, and nothing anywhere says why.
  withCredentials: true,
  // NOT optional, and the reason is easy to miss: axios attaches the X-XSRF-TOKEN header
  // from the XSRF-TOKEN cookie only when this is set or the request is same-origin. The NOC
  // baseURL is cross-origin, so without it every mutating request answers 403 — and no Go
  // test can catch it, because backend tests set the header themselves. This is exactly how
  // the same defect survived a review round on the gateways.
  withXSRFToken: true,
});

/** Marks a request that already went through one post-401 renewal attempt. */
type RetriableConfig = InternalAxiosRequestConfig & { _retriedAfterRefresh?: boolean };

// No request interceptor attaches a credential any more: the browser attaches the cookie on
// its own, and there is no token in the page to attach.

// A 401 means the session cookie was rejected — expired, revoked, or the backend restarted.
// Renew once and replay the request; if that fails the session is over and the portal
// returns to the login screen.
httpClient.interceptors.response.use(
  (response) => {
    useFreshnessStore.getState().markSuccess();
    return response;
  },
  async (error) => {
    useFreshnessStore.getState().markError();
    const config = error?.config as RetriableConfig | undefined;
    if (error?.response?.status !== 401 || !config || config._retriedAfterRefresh) {
      return Promise.reject(error);
    }
    // The login and refresh routes are where a 401 is the ANSWER, not a stale session.
    // Retrying them would loop: a failed refresh would trigger another refresh.
    if (typeof config.url === "string" && /\/auth\/(login|refresh)$/.test(config.url)) {
      return Promise.reject(error);
    }

    config._retriedAfterRefresh = true;
    const renewed = await refreshSession();
    if (!renewed) {
      notifySessionExpired();
      return Promise.reject(error);
    }

    // Replayed without touching headers: the renewed cookies are already in the jar, and
    // the CSRF header is re-attached by axios from the rewritten XSRF-TOKEN cookie.
    return httpClient.request(config);
  },
);

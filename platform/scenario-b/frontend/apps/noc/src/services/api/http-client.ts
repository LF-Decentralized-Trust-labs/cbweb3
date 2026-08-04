// SPDX-License-Identifier: Apache-2.0

import axios, { type InternalAxiosRequestConfig } from "axios";
import { useFreshnessStore } from "../../stores/freshness.store";
import { ensureFreshToken, notifySessionExpired, refreshAccessToken } from "./token";

export const httpClient = axios.create({
  baseURL: import.meta.env.VITE_NOC_BACKEND_URL ?? "/api/v1",
  withCredentials: false,
});

/** Marks a request that already went through one post-401 renewal attempt. */
type RetriableConfig = InternalAxiosRequestConfig & { _retriedAfterRefresh?: boolean };

// Attach the Keycloak access token, renewing it first when it is about to expire.
httpClient.interceptors.request.use(async (config) => {
  const token = await ensureFreshToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// A 401 that survives the proactive renewal means the token was rejected anyway (clock
// skew, revoked session, backend restart). Renew once and replay the request; if that
// fails the session is over and the portal returns to the login screen.
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

    config._retriedAfterRefresh = true;
    const token = await refreshAccessToken();
    if (!token) {
      notifySessionExpired();
      return Promise.reject(error);
    }

    config.headers.Authorization = `Bearer ${token}`;
    return httpClient.request(config);
  },
);

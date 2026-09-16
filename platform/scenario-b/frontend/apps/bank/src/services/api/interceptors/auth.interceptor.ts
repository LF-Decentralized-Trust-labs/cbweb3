// SPDX-License-Identifier: Apache-2.0

import axios, { AxiosError, type AxiosInstance, type AxiosResponse, type InternalAxiosRequestConfig } from "axios";
import { isRelayConfigurationFault, isTrustRejection } from "../trust-errors";

const REFRESH_PATH = "/auth/refresh";
const API_VERSION_PATH_RE = /\/api\/v[0-9]+$/i;

/** pathOf strips the query string, so the same endpoint compares equal across calls. */
function pathOf(url?: string): string {
  return (url ?? "").split("?")[0] ?? "";
}

type RetriableRequestConfig = InternalAxiosRequestConfig & {
  _isRetrying?: boolean;
};

type ApiErrorBody = {
  error?: string;
};

function isBridgeClaimsError(error: AxiosError<ApiErrorBody>, requestConfig?: RetriableRequestConfig): boolean {
  const requestUrl = requestConfig?.url ?? "";
  const responseError = error.response?.data?.error ?? "";
  return requestUrl.includes("/bridge/") && responseError === "missing authenticated claims";
}

function resolveV1BaseURL(baseURL?: string): string {
  const normalizedBaseURL = (baseURL ?? "").replace(/\/+$/, "");

  if (normalizedBaseURL === "") {
    return "/api/v1";
  }

  if (API_VERSION_PATH_RE.test(normalizedBaseURL)) {
    const apiRoot = normalizedBaseURL.replace(API_VERSION_PATH_RE, "");
    return `${apiRoot}/api/v1`;
  }

  return `${normalizedBaseURL}/api/v1`;
}

async function onTrustRestored(response: AxiosResponse) {
  const { useTrustStore } = await import("../../../stores/trust.store");
  // The store decides: only a path the central bank had refused proves the channel works again. This
  // layer cannot tell, and guessing here is what made two locally-served balance calls retire a notice
  // that three rejected payment calls had just raised.
  useTrustStore.getState().noteSuccess(pathOf(response.config.url));
  return response;
}

async function onTrustRejected(error: AxiosError, client: AxiosInstance) {
  const { useTrustStore } = await import("../../../stores/trust.store");
  const config = error.config;
  // The probe replays this exact request through the SAME instance, so it inherits the baseURL,
  // credentials and interceptors of the request that was refused; a request rebuilt by hand would
  // drop them and prove nothing. Whether it may be replayed at all is the store's call, which is why
  // the method goes with it: rejected paths include writes, and repeating one of those because an
  // operator pressed "Recheck" would move value.
  const probe = config ? () => client.request(config) : undefined;
  void useTrustStore.getState().reportRejection(pathOf(config?.url), config?.method, probe);
  return Promise.reject(error);
}

async function onRelayConfigurationFault(error: AxiosError, client: AxiosInstance) {
  const { useTrustStore } = await import("../../../stores/trust.store");
  const config = error.config;
  // Same rule as onTrustRejected: the probe replays through this instance, and the method is what
  // lets the store refuse to repeat a write.
  const probe = config ? () => client.request(config) : undefined;
  void useTrustStore.getState().reportConfigurationFault(pathOf(config?.url), config?.method, probe);
  return Promise.reject(error);
}

export function attachAuthInterceptor(httpClient: AxiosInstance) {
  httpClient.interceptors.response.use(
    (response) => onTrustRestored(response),
    async (error: AxiosError) => {
      // The central bank refusing our identity is not an expired session. Refreshing succeeds — the
      // cookie is valid — the retry is rejected again, and the second 401 used to reach the logout
      // branch below, ejecting the operator to the login screen over a failure that has nothing to do
      // with their session. Report it instead, so every screen can explain the real cause.
      if (isTrustRejection(error)) {
        return onTrustRejected(error, httpClient);
      }

      // Nor is the relay credential being refused. The session is valid here too, so the refresh
      // would succeed and the retry would fail identically — the same ejection, reached through a
      // different door. It is checked before the status test because one of these is a 503.
      if (isRelayConfigurationFault(error)) {
        return onRelayConfigurationFault(error, httpClient);
      }

      if (!error.response || error.response.status !== 401) {
        return Promise.reject(error);
      }

      const typedError = error as AxiosError<ApiErrorBody>;
      const requestConfig = typedError.config as RetriableRequestConfig | undefined;

      if (isBridgeClaimsError(typedError, requestConfig)) {
        return Promise.reject(error);
      }

      if (!requestConfig || requestConfig._isRetrying || requestConfig.url?.includes(REFRESH_PATH)) {
        const { useAuthStore } = await import("../../../stores");
        useAuthStore.getState().forceLogout();
        return Promise.reject(error);
      }

      requestConfig._isRetrying = true;

      try {
        await axios.post(
          REFRESH_PATH,
          {},
          {
            baseURL: resolveV1BaseURL(httpClient.defaults.baseURL),
            withCredentials: true,
          },
        );

        return httpClient(requestConfig);
      } catch (refreshError) {
        const { useAuthStore } = await import("../../../stores");
        useAuthStore.getState().forceLogout();
        return Promise.reject(refreshError);
      }
    },
  );
}
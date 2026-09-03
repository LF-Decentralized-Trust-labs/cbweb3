// SPDX-License-Identifier: Apache-2.0

import axios, { AxiosError, type AxiosInstance, type InternalAxiosRequestConfig } from "axios";

const REFRESH_PATH = "/auth/refresh";
const API_VERSION_PATH_RE = /\/api\/v[0-9]+$/i;

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

export function attachAuthInterceptor(httpClient: AxiosInstance) {
  httpClient.interceptors.response.use(
    (response) => response,
    async (error: AxiosError) => {
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
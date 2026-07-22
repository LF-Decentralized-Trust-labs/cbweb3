// SPDX-License-Identifier: Apache-2.0

import axios, { AxiosError, type AxiosInstance, type InternalAxiosRequestConfig } from "axios";

const REFRESH_PATH = "/auth/refresh";

type RetriableRequestConfig = InternalAxiosRequestConfig & {
  _isRetrying?: boolean;
};

export function attachAuthInterceptor(httpClient: AxiosInstance) {
  httpClient.interceptors.response.use(
    (response) => response,
    async (error: AxiosError) => {
      if (!error.response || error.response.status !== 401) {
        return Promise.reject(error);
      }

      const requestConfig = error.config as RetriableRequestConfig | undefined;
      if (!requestConfig || requestConfig._isRetrying || requestConfig.url?.includes(REFRESH_PATH)) {
        const { useAuthStore } = await import("../../../stores");
        useAuthStore.getState().forceLogout();
        return Promise.reject(error);
      }

      requestConfig._isRetrying = true;

      try {
        // The refresh endpoint lives only under /api/v1/auth — derive it from the
        // client's baseURL so a v2 client (baseURL /api/v2) does not POST to a
        // non-existent /api/v2/auth/refresh (404 → spurious forceLogout).
        const authBaseUrl = (httpClient.defaults.baseURL ?? "").replace(/\/api\/v\d+$/i, "/api/v1");
        await axios.post(
          REFRESH_PATH,
          {},
          {
            baseURL: authBaseUrl,
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

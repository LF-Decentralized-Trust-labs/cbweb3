// SPDX-License-Identifier: Apache-2.0

import axios, { type AxiosError } from "axios";
import { attachAuthInterceptor } from "./interceptors/auth.interceptor";

const apiBaseUrl = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

export const httpClient = axios.create({
  baseURL: apiBaseUrl,
  withCredentials: true,
});

attachAuthInterceptor(httpClient);

// Extract backend `error` field from non-401 error responses so toast messages are meaningful.
httpClient.interceptors.response.use(
  (response) => response,
  (error: AxiosError<{ error?: string; error_code?: string }>) => {
    const apiError = error.response?.data?.error;
    if (apiError && error.response?.status !== 401) {
      return Promise.reject(new Error(apiError));
    }
    return Promise.reject(error);
  },
);
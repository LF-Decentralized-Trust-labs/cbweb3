// SPDX-License-Identifier: Apache-2.0

import axios from "axios";
import { attachAuthInterceptor } from "./interceptors/auth.interceptor";

const apiBaseUrl = (import.meta.env.VITE_API_BASE_URL ?? "/api/v1").replace(/\/+$/, "");
const apiBaseRoot = apiBaseUrl.replace(/\/api\/v[0-9]+$/i, "");
const apiBaseV1 = /\/api\/v[0-9]+$/i.test(apiBaseUrl) ? apiBaseUrl : `${apiBaseRoot}/api/v1`;
const apiBaseV2 = `${apiBaseRoot}/api/v2`;

export const httpClient = axios.create({
  baseURL: apiBaseV1,
  withCredentials: true,
});

export const httpClientV2 = axios.create({
  baseURL: apiBaseV2,
  withCredentials: true,
});

// Silent-refresh a 401 once and retry (both v1 and v2 clients refresh via v1).
attachAuthInterceptor(httpClient);
attachAuthInterceptor(httpClientV2);

// SPDX-License-Identifier: Apache-2.0

import axios from "axios";
import { attachAuthInterceptor } from "./interceptors/auth.interceptor";

const apiBaseUrl = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

export const httpClient = axios.create({
  baseURL: `${apiBaseUrl}`,
  withCredentials: true,
  withXSRFToken: true,
});

attachAuthInterceptor(httpClient);

import axios from "axios";
import { attachAuthInterceptor } from "./interceptors/auth.interceptor";

const apiBaseUrl = import.meta.env.VITE_API_URL ?? "http://localhost:8080";
const normalizedApiBaseUrl = apiBaseUrl.replace(/\/+$/, "");
const apiBaseRoot = normalizedApiBaseUrl.replace(/\/api\/v[0-9]+$/i, "");

const apiBaseV1 = /\/api\/v[0-9]+$/i.test(normalizedApiBaseUrl)
  ? normalizedApiBaseUrl
  : `${apiBaseRoot}/api/v1`;

export const apiBaseV2 = `${apiBaseRoot}/api/v2`;

export const httpClient = axios.create({
  baseURL: apiBaseV1,
  withCredentials: true,
});

export const httpClientV2 = axios.create({
  baseURL: apiBaseV2,
  withCredentials: true,
});

attachAuthInterceptor(httpClient);
attachAuthInterceptor(httpClientV2);
import axios from "axios";
import { attachAuthInterceptor } from "./interceptors/auth.interceptor";

const apiBaseUrl = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

export const httpClient = axios.create({
  baseURL: apiBaseUrl,
  timeout: 15_000,
  withCredentials: true,
});

attachAuthInterceptor(httpClient);
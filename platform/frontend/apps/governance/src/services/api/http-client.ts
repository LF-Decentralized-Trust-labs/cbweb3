import axios from "axios";
import { attachAuthInterceptor } from "./interceptors/auth.interceptor";

export const useMocks = import.meta.env.VITE_USE_MOCKS !== "false";
const apiBaseUrl = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

export const httpClient = axios.create({
  baseURL: `${apiBaseUrl}`,
  withCredentials: true,
});

attachAuthInterceptor(httpClient);

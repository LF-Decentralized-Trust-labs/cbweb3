import axios from "axios";

export const useMocks = import.meta.env.VITE_USE_MOCKS !== "false";

export const httpClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080",
  timeout: 12_000,
});

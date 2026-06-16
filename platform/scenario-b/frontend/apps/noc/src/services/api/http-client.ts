import axios from "axios";

export const httpClient = axios.create({
  baseURL: import.meta.env.VITE_NOC_BACKEND_URL ?? "/api/v1",
  withCredentials: false,
});

// Attach the Keycloak access token to every request when available.
httpClient.interceptors.request.use((config) => {
  const token = localStorage.getItem("noc_access_token");
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

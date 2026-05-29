import axios from "axios";

export const httpClient = axios.create({
  baseURL: import.meta.env.VITE_NOC_BACKEND_URL ?? "/api/v1",
  withCredentials: false,
});

// Attach Keycloak access token from localStorage on every request.
httpClient.interceptors.request.use((config) => {
  const token = localStorage.getItem("noc_access_token");
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

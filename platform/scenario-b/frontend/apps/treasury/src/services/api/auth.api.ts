import type { LoginResponse } from "../../types";
import { httpClient } from "./http-client";

export const authApi = {
  login: async (username: string, password: string): Promise<LoginResponse> => {
    const response = await httpClient.post<LoginResponse>("/auth/login", { username, password });
    return response.data;
  },
  me: async (): Promise<LoginResponse> => {
    const response = await httpClient.get<LoginResponse>("/auth/me");
    return response.data;
  },
  logout: async (): Promise<void> => {
    await httpClient.post("/auth/logout", {});
  },
};

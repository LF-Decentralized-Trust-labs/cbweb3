import type { LoginResponse, UserProfile } from "../../types";
import { httpClient } from "./http-client";

export const authApi = {
  login: async (clientId: string, clientSecret: string): Promise<LoginResponse> => {
    const response = await httpClient.post<LoginResponse>("/auth/login", {
      clientId,
      clientSecret,
    });

    return response.data;
  },
  me: async (): Promise<UserProfile> => {
    const response = await httpClient.get<UserProfile>("/auth/me");
    return response.data;
  },
  refresh: async () => {
    await httpClient.post("/auth/refresh", {});
  },
  logout: async () => {
    await httpClient.post("/auth/logout", {});
  },
};

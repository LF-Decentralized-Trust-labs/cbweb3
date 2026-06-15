import type { MeResponse } from "../../types";
import { httpClient } from "./http-client";

export const authApi = {
  login: async (clientId: string, clientSecret: string): Promise<void> => {
    await httpClient.post("/auth/login", { clientId, clientSecret });
  },
  me: async (): Promise<MeResponse> => {
    const response = await httpClient.get<MeResponse>("/auth/me");
    return response.data;
  },
  logout: async (): Promise<void> => {
    await httpClient.post("/auth/logout", {});
  },
};

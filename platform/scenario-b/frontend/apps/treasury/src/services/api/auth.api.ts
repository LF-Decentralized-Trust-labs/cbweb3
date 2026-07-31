// SPDX-License-Identifier: Apache-2.0

import type { MeResponse } from "../../types";
import { httpClient } from "./http-client";

export const authApi = {
  login: async (clientId: string, clientSecret: string): Promise<{ expiresIn: number }> => {
    const response = await httpClient.post<{ expiresIn: number }>("/auth/login", { clientId, clientSecret });
    return response.data;
  },
  me: async (): Promise<MeResponse> => {
    const response = await httpClient.get<MeResponse>("/auth/me");
    return response.data;
  },
  refresh: async (): Promise<{ expiresIn: number }> => {
    const response = await httpClient.post<{ expiresIn: number }>("/auth/refresh", {});
    return response.data;
  },
  logout: async (): Promise<void> => {
    await httpClient.post("/auth/logout", {});
  },
};

// SPDX-License-Identifier: Apache-2.0

import type { LoginResponse, UserProfile } from "../../types";
import { httpClient } from "./http-client";

export const authApi = {
  login: async (username: string, password: string): Promise<LoginResponse> => {
    const response = await httpClient.post<LoginResponse>("/auth/login", { clientId: username, clientSecret: password });
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

import type { LoginResponse } from "../../types";
import { mockDb } from "../mocks/mock-db";
import { httpClient, useMocks } from "./http-client";

export const authApi = {
  login: async (username: string, password: string): Promise<LoginResponse> => {
    if (useMocks) {
      return mockDb.login(username, password);
    }
    const response = await httpClient.post<LoginResponse>("/api/v1/auth/login", { username, password });
    return response.data;
  },
  logout: async () => {
    return Promise.resolve();
  },
};

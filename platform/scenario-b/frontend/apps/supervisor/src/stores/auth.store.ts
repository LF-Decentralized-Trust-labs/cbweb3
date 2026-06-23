// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { authApi } from "../services/api";
import type { SupervisorUser } from "../types";

type AuthState = {
  user: SupervisorUser | null;
  isAuthenticated: boolean;
  initialized: boolean;
  status: "idle" | "loading" | "error";
  error: string | null;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  checkSession: () => Promise<void>;
};

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isAuthenticated: false,
  initialized: false,
  status: "idle",
  error: null,
  login: async (username, password) => {
    set({ status: "loading", error: null });
    try {
      const response = await authApi.login(username, password);
      set({ user: response.user, isAuthenticated: true, initialized: true, status: "idle" });
    } catch (error) {
      set({
        status: "error",
        error: error instanceof Error ? error.message : "Unable to login",
        initialized: true,
        isAuthenticated: false,
      });
    }
  },
  logout: async () => {
    try {
      await authApi.logout();
    } finally {
      set({ user: null, isAuthenticated: false, initialized: true, status: "idle", error: null });
    }
  },
  checkSession: async () => {
    set({ status: "loading", error: null });
    try {
      const user = await authApi.me();
      set({ user, isAuthenticated: true, initialized: true, status: "idle" });
    } catch {
      set({ user: null, isAuthenticated: false, initialized: true, status: "idle", error: null });
    }
  },
}));

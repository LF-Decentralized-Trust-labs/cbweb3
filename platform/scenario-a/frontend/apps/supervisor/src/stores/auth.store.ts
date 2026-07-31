// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { authApi } from "../services/api";
import { cancelTokenRefresh, scheduleTokenRefresh } from "../services/api/token-refresh";
import type { SupervisorUser } from "../types";

type AuthState = {
  user: SupervisorUser | null;
  isAuthenticated: boolean;
  initialized: boolean;
  status: "idle" | "loading" | "error";
  error: string | null;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  forceLogout: () => void;
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
      // Keep the short-lived access token renewed for the whole session.
      scheduleTokenRefresh(response.sessionTimeoutSeconds);
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
    cancelTokenRefresh();
    try {
      await authApi.logout();
    } finally {
      set({ user: null, isAuthenticated: false, initialized: true, status: "idle", error: null });
    }
  },
  forceLogout: () => {
    cancelTokenRefresh();
    set({ user: null, isAuthenticated: false, initialized: true, status: "idle", error: null });
  },
  checkSession: async () => {
    set({ status: "loading", error: null });
    try {
      const user = await authApi.me();
      set({ user, isAuthenticated: true, initialized: true, status: "idle" });
      // Session restored on load — (re)arm proactive refresh (best-effort).
      void authApi
        .refresh()
        .then((result) => scheduleTokenRefresh(result.expiresIn))
        .catch(() => {});
    } catch {
      cancelTokenRefresh();
      set({ user: null, isAuthenticated: false, initialized: true, status: "idle", error: null });
    }
  },
}));

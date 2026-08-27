// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { SUPERVISOR_UNAUTHORIZED_MESSAGE, hasSupervisorAccess } from "../auth/authorization";
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
      // Authenticated is not authorised: the gateway refuses every supervisor route to a
      // session without ROLE_SUPERVISOR, so admitting it here only produces a portal that
      // renders and then 403s on each request.
      const authorized = hasSupervisorAccess(response.user);
      set({
        user: response.user,
        isAuthenticated: authorized,
        initialized: true,
        status: "idle",
        error: authorized ? null : SUPERVISOR_UNAUTHORIZED_MESSAGE,
      });
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
      const authorized = hasSupervisorAccess(user);
      set({
        user,
        isAuthenticated: authorized,
        initialized: true,
        status: "idle",
        error: authorized ? null : SUPERVISOR_UNAUTHORIZED_MESSAGE,
      });
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

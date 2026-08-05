// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { authApi } from "../services/api";
import { setSessionExpiredHandler } from "../services/api/token";
import type { AsyncStatus, SysAdminUser } from "../types";

type AuthState = {
  user: SysAdminUser | null;
  isAuthenticated: boolean;
  initialized: boolean;
  status: AsyncStatus;
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
        isAuthenticated: false,
        initialized: true,
      });
    }
  },
  logout: async () => {
    await authApi.logout();
    set({ user: null, isAuthenticated: false, initialized: true, status: "idle", error: null });
  },
  checkSession: async () => {
    try {
      const response = await authApi.me();
      set({ user: response.user, isAuthenticated: true, initialized: true, status: "idle", error: null });
    } catch {
      set({ user: null, isAuthenticated: false, initialized: true, status: "idle", error: null });
    }
  },
}));

// When renewal is no longer possible the API layer reports the session as over; dropping
// isAuthenticated makes ProtectedRoute send the operator to /login instead of leaving the
// portal showing stale data behind failing requests.
setSessionExpiredHandler(() => {
  useAuthStore.setState({
    user: null,
    isAuthenticated: false,
    initialized: true,
    status: "idle",
    error: "Session expired. Sign in again.",
  });
});

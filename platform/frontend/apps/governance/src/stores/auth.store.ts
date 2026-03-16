import { create } from "zustand";
import { authApi } from "../services/api";
import type { AsyncStatus, GovernanceUser } from "../types";

type AuthState = {
  user: GovernanceUser | null;
  token: string | null;
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
  token: null,
  isAuthenticated: false,
  initialized: false,
  status: "idle",
  error: null,
  login: async (username, password) => {
    set({ status: "loading", error: null });
    try {
      const response = await authApi.login(username, password);
      set({
        user: response.user,
        token: response.token,
        isAuthenticated: true,
        initialized: true,
        status: "idle",
      });
    } catch (error) {
      set({
        user: null,
        token: null,
        isAuthenticated: false,
        initialized: true,
        status: "error",
        error: error instanceof Error ? error.message : "Unable to login",
      });
    }
  },
  logout: async () => {
    await authApi.logout();
    set({ user: null, token: null, isAuthenticated: false, initialized: true, status: "idle", error: null });
  },
  checkSession: async () => {
    set({ user: null, token: null, isAuthenticated: false, initialized: true, status: "idle", error: null });
  },
}));

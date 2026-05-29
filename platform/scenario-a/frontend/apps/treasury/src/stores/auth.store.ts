import { create } from "zustand";
import { authApi } from "../services/api";
import type { AsyncStatus, UserProfile } from "../types";
import { TREASURY_UNAUTHORIZED_MESSAGE, hasTreasuryAccess } from "../auth/authorization";

type AuthState = {
  profile: UserProfile | null;
  isAuthenticated: boolean;
  initialized: boolean;
  status: AsyncStatus;
  error: string | null;
  login: (clientId: string, clientSecret: string) => Promise<void>;
  logout: () => Promise<void>;
  forceLogout: () => void;
  checkSession: () => Promise<void>;
};

export const useAuthStore = create<AuthState>((set) => ({
  profile: null,
  isAuthenticated: false,
  initialized: false,
  status: "idle",
  error: null,
  login: async (clientId, clientSecret) => {
    set({ status: "loading", error: null });
    try {
      const loginResponse = await authApi.login(clientId, clientSecret);
      if ("nonce" in loginResponse) {
        throw new Error("PKI authentication is required for this account and is not available in Treasury login.");
      }

      const profile = await authApi.me();
      const authorized = hasTreasuryAccess(profile);
      set({
        profile,
        isAuthenticated: authorized,
        initialized: true,
        status: "idle",
        error: authorized ? null : TREASURY_UNAUTHORIZED_MESSAGE,
      });
    } catch (error) {
      set({
        profile: null,
        isAuthenticated: false,
        initialized: true,
        status: "error",
        error: error instanceof Error ? error.message : "Unable to login",
      });
    }
  },
  logout: async () => {
    try {
      await authApi.logout();
    } finally {
      set({ profile: null, isAuthenticated: false, initialized: true, status: "idle", error: null });
    }
  },
  forceLogout: () => {
    set({ profile: null, isAuthenticated: false, initialized: true, status: "idle", error: null });
  },
  checkSession: async () => {
    set({ status: "loading", error: null });
    try {
      const profile = await authApi.me();
      const authorized = hasTreasuryAccess(profile);
      set({
        profile,
        isAuthenticated: authorized,
        initialized: true,
        status: "idle",
        error: authorized ? null : TREASURY_UNAUTHORIZED_MESSAGE,
      });
    } catch {
      set({
        profile: null,
        isAuthenticated: false,
        initialized: true,
        status: "idle",
        error: null,
      });
    }
  },
}));

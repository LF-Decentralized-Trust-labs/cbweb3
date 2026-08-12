// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { authApi } from "../services/api";
import { cancelTokenRefresh, scheduleTokenRefresh } from "../services/api/token-refresh";
import type { AsyncStatus, UserProfile } from "../types";
import { BANK_UNAUTHORIZED_MESSAGE, hasBankAccess } from "../auth/authorization";
import { useTrustStore } from "./trust.store";

type AuthState = {
  profile: UserProfile | null;
  isAuthenticated: boolean;
  initialized: boolean;
  status: AsyncStatus;
  error: string | null;
  login: (clientId: string, clientSecret: string) => Promise<void>;
  logout: () => Promise<void>;
  forceLogout: () => void;
  bindWallet: (walletAddress: string) => Promise<void>;
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
        throw new Error("PKI authentication is not supported in the Bank Portal.");
      }

      // Keep the short-lived access token renewed for the whole session.
      scheduleTokenRefresh(loginResponse.expiresIn);

      const profile = await authApi.me();
      const authorized = hasBankAccess(profile);
      set({
        profile,
        isAuthenticated: authorized,
        initialized: true,
        status: "idle",
        error: authorized ? null : BANK_UNAUTHORIZED_MESSAGE,
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
    cancelTokenRefresh();
    // The notice belongs to the institution that was signed in; carrying it into the next session
    // would accuse a different bank of not being registered.
    useTrustStore.getState().clear();
    try {
      await authApi.logout();
    } finally {
      set({
        profile: null,
        isAuthenticated: false,
        initialized: true,
        status: "idle",
        error: null,
      });
    }
  },
  forceLogout: () => {
    cancelTokenRefresh();
    useTrustStore.getState().clear();
    set({
      profile: null,
      isAuthenticated: false,
      initialized: true,
      status: "idle",
      error: null,
    });
  },

  bindWallet: async (walletAddress) => {
    void walletAddress;
    // TODO: implement against real wallet API when available.
  },
  checkSession: async () => {
    set({ status: "loading", error: null });

    try {
      const profile = await authApi.me();
      const authorized = hasBankAccess(profile);
      set({
        profile,
        isAuthenticated: authorized,
        initialized: true,
        status: "idle",
        error: authorized ? null : BANK_UNAUTHORIZED_MESSAGE,
      });
      // Session restored on load — (re)arm proactive refresh (best-effort).
      void authApi
        .refresh()
        .then((result) => scheduleTokenRefresh(result.expiresIn))
        .catch(() => {});
    } catch {
      cancelTokenRefresh();
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

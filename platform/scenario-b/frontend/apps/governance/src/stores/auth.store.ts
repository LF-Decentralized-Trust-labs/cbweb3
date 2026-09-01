// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { authApi } from "../services/api";
import { cancelTokenRefresh, scheduleTokenRefresh } from "../services/api/token-refresh";
import type { AsyncStatus, UserProfile } from "../types";
import { GOVERNANCE_UNAUTHORIZED_MESSAGE, hasPortalAccess } from "../auth/authorization";
import { loginErrorMessage } from "@cbweb3/ui";

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
        throw new Error("PKI authentication is required for this account and is not available in Governance login.");
      }

      // Keep the short-lived access token renewed for the whole session.
      scheduleTokenRefresh(loginResponse.expiresIn);

      const profile = await authApi.me();
      const authorized = hasPortalAccess(profile);
      set({
        profile,
        isAuthenticated: authorized,
        initialized: true,
        status: "idle",
        error: authorized ? null : GOVERNANCE_UNAUTHORIZED_MESSAGE,
      });
    } catch (error) {
      set({
        profile: null,
        isAuthenticated: false,
        initialized: true,
        status: "error",
        // The gateway names the cause in the body; axios's own message is only "Request failed
        // with status code N". Shared so the five portals cannot drift apart on this copy.
        error: loginErrorMessage(error),
      });
    }
  },
  logout: async () => {
    cancelTokenRefresh();
    try {
      await authApi.logout();
    } finally {
      set({ profile: null, isAuthenticated: false, initialized: true, status: "idle", error: null });
    }
  },
  forceLogout: () => {
    cancelTokenRefresh();
    set({ profile: null, isAuthenticated: false, initialized: true, status: "idle", error: null });
  },
  checkSession: async () => {
    set({ status: "loading", error: null });
    try {
      const profile = await authApi.me();
      const authorized = hasPortalAccess(profile);
      set({
        profile,
        isAuthenticated: authorized,
        initialized: true,
        status: "idle",
        error: authorized ? null : GOVERNANCE_UNAUTHORIZED_MESSAGE,
      });
      // Session restored on load — (re)arm proactive refresh (best-effort).
      void authApi
        .refresh()
        .then((result) => scheduleTokenRefresh(result.expiresIn))
        .catch(() => {});
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

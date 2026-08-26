// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { authApi } from "../services/api";
import { cancelTokenRefresh, scheduleTokenRefresh } from "../services/api/token-refresh";
import type { AsyncStatus, MeResponse, TreasuryUser } from "../types";
import { TREASURY_UNAUTHORIZED_MESSAGE, hasTreasuryAccess } from "../auth/authorization";

type AuthState = {
  user: TreasuryUser | null;
  isAuthenticated: boolean;
  initialized: boolean;
  status: AsyncStatus;
  error: string | null;
  login: (clientId: string, clientSecret: string) => Promise<void>;
  logout: () => Promise<void>;
  forceLogout: () => void;
  checkSession: () => Promise<void>;
};

function meToUser(me: MeResponse): TreasuryUser {
  return {
    id: me.subject,
    name: me.subject,
    institutionId: me.bankId ?? "",
    roles: me.roles ?? [],
    // Fixed label, not a claim — every session mapped here became a "TREASURY" user
    // regardless of what its token actually carried. Authorization reads `roles`.
    role: "TREASURY",
    walletAddress: me.wallet ?? "",
    authorizedIssuer: me.roles.some((r) => r.toLowerCase().includes("treasury") || r.toLowerCase().includes("issuer")),
  };
}

// Authenticated is not authorised: the gateway refuses the treasury routes to a session
// without ROLE_TREASURY, so admitting it here only produces a portal that renders and then
// 403s on every request. The login screen shows `error`.
function authorizedState(user: TreasuryUser) {
  const authorized = hasTreasuryAccess(user);
  return {
    user,
    isAuthenticated: authorized,
    initialized: true,
    status: "idle" as const,
    error: authorized ? null : TREASURY_UNAUTHORIZED_MESSAGE,
  };
}

export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isAuthenticated: false,
  initialized: false,
  status: "idle",
  error: null,
  login: async (clientId, clientSecret) => {
    set({ status: "loading", error: null });
    try {
      const { expiresIn } = await authApi.login(clientId, clientSecret);
      // Keep the short-lived access token renewed for the whole session.
      scheduleTokenRefresh(expiresIn);
      const me = await authApi.me();
      set(authorizedState(meToUser(me)));
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
    cancelTokenRefresh();
    await authApi.logout();
    set({ user: null, isAuthenticated: false, initialized: true, status: "idle", error: null });
  },
  forceLogout: () => {
    cancelTokenRefresh();
    set({ user: null, isAuthenticated: false, initialized: true, status: "idle", error: null });
  },
  checkSession: async () => {
    try {
      const me = await authApi.me();
      set(authorizedState(meToUser(me)));
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

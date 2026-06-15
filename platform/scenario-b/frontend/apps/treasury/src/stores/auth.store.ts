import { create } from "zustand";
import { authApi } from "../services/api";
import type { AsyncStatus, MeResponse, TreasuryUser } from "../types";

type AuthState = {
  user: TreasuryUser | null;
  isAuthenticated: boolean;
  initialized: boolean;
  status: AsyncStatus;
  error: string | null;
  login: (clientId: string, clientSecret: string) => Promise<void>;
  logout: () => Promise<void>;
  checkSession: () => Promise<void>;
};

function meToUser(me: MeResponse): TreasuryUser {
  return {
    id: me.subject,
    name: me.subject,
    institutionId: me.bankId ?? "",
    role: "TREASURY",
    walletAddress: me.wallet ?? "",
    authorizedIssuer: me.roles.some((r) => r.toLowerCase().includes("treasury") || r.toLowerCase().includes("issuer")),
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
      await authApi.login(clientId, clientSecret);
      const me = await authApi.me();
      set({ user: meToUser(me), isAuthenticated: true, initialized: true, status: "idle" });
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
      const me = await authApi.me();
      set({ user: meToUser(me), isAuthenticated: true, initialized: true, status: "idle" });
    } catch {
      set({ user: null, isAuthenticated: false, initialized: true, status: "idle", error: null });
    }
  },
}));

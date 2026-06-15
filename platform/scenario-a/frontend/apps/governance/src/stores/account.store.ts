import { create } from "zustand";
import { governanceApi } from "../services/api";
import type { AccountEntry, AsyncStatus, FreezePayload } from "../types";

type AccountStore = {
  accounts: AccountEntry[];
  status: AsyncStatus;
  error: string | null;
  fetch: () => Promise<void>;
  freeze: (payload: FreezePayload) => Promise<void>;
  unfreeze: (payload: FreezePayload) => Promise<void>;
};

export const useAccountStore = create<AccountStore>((set, get) => ({
  accounts: [],
  status: "idle",
  error: null,
  fetch: async () => {
    set({ status: "loading", error: null });
    try {
      const accounts = await governanceApi.listAccounts();
      set({ accounts, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load accounts" });
    }
  },
  freeze: async (payload) => {
    set({ status: "loading", error: null });
    try {
      await governanceApi.freezeAccount(payload);
      await get().fetch();
      set({ status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to freeze account" });
    }
  },
  unfreeze: async (payload) => {
    set({ status: "loading", error: null });
    try {
      await governanceApi.unfreezeAccount(payload);
      await get().fetch();
      set({ status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to unfreeze account" });
    }
  },
}));

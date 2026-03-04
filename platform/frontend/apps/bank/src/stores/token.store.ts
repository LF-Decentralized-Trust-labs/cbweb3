import { create } from "zustand";
import { tokenApi } from "../services/api";
import type { MintRequest, TokenBalance, TokenTransaction, TransferRequest } from "../types";

type TokenState = {
  balance: TokenBalance | null;
  transactions: TokenTransaction[];
  status: "idle" | "loading" | "error";
  error: string | null;
  fetch: () => Promise<void>;
  mint: (payload: MintRequest) => Promise<void>;
  transfer: (payload: TransferRequest) => Promise<void>;
};

export const useTokenStore = create<TokenState>((set) => ({
  balance: null,
  transactions: [],
  status: "idle",
  error: null,
  fetch: async () => {
    set({ status: "loading", error: null });
    try {
      const [balance, transactions] = await Promise.all([tokenApi.getBalance(), tokenApi.getTransactions()]);
      set({ balance, transactions, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load balances" });
    }
  },
  mint: async (payload) => {
    set({ status: "loading", error: null });
    try {
      await tokenApi.mint(payload);
      const [balance, transactions] = await Promise.all([tokenApi.getBalance(), tokenApi.getTransactions()]);
      set({ balance, transactions, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to mint" });
    }
  },
  transfer: async (payload) => {
    set({ status: "loading", error: null });
    try {
      await tokenApi.transfer(payload);
      const [balance, transactions] = await Promise.all([tokenApi.getBalance(), tokenApi.getTransactions()]);
      set({ balance, transactions, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to transfer" });
    }
  },
}));

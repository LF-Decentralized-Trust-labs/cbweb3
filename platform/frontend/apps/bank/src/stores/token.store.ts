import { create } from "zustand";
import { tokenApi } from "../services/api";
import type { OnRampRequest, OnRampRequestPayload, TokenBalance, TokenTransaction, TransferRequest } from "../types";

type TokenState = {
  balance: TokenBalance | null;
  transactions: TokenTransaction[];
  onRampRequests: OnRampRequest[];
  status: "idle" | "loading" | "error";
  error: string | null;
  fetch: () => Promise<void>;
  transfer: (payload: TransferRequest) => Promise<void>;
  requestOnRamp: (payload: OnRampRequestPayload) => Promise<void>;
};

export const useTokenStore = create<TokenState>((set) => ({
  balance: null,
  transactions: [],
  onRampRequests: [],
  status: "idle",
  error: null,
  fetch: async () => {
    set({ status: "loading", error: null });
    try {
      const [balance, transactions, onRampRequests] = await Promise.all([
        tokenApi.getBalance(),
        tokenApi.getTransactions(),
        tokenApi.getOnRampRequests(),
      ]);
      set({ balance, transactions, onRampRequests, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load balances" });
    }
  },
  transfer: async (payload) => {
    set({ status: "loading", error: null });
    try {
      await tokenApi.transfer(payload);
      const [balance, transactions, onRampRequests] = await Promise.all([
        tokenApi.getBalance(),
        tokenApi.getTransactions(),
        tokenApi.getOnRampRequests(),
      ]);
      set({ balance, transactions, onRampRequests, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to transfer" });
    }
  },
  requestOnRamp: async (payload) => {
    set({ status: "loading", error: null });
    try {
      await tokenApi.requestOnRamp(payload);
      const [balance, transactions, onRampRequests] = await Promise.all([
        tokenApi.getBalance(),
        tokenApi.getTransactions(),
        tokenApi.getOnRampRequests(),
      ]);
      set({ balance, transactions, onRampRequests, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to submit liquidity request" });
    }
  },
}));

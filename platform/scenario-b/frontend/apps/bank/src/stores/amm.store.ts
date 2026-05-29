import { create } from "zustand";
import { ammApi } from "../services/api";
import type { AMMQuoteRequest, AMMQuoteResponse, AMMPoolStatus } from "../types";

type AMMState = {
  quote: AMMQuoteResponse | null;
  pool: AMMPoolStatus | null;
  status: "idle" | "loading" | "error";
  error: string | null;
  refreshPool: () => Promise<void>;
  getQuote: (payload: AMMQuoteRequest) => Promise<void>;
  swap: (payload: AMMQuoteRequest) => Promise<void>;
};

export const useAmmStore = create<AMMState>((set) => ({
  quote: null,
  pool: null,
  status: "idle",
  error: null,
  refreshPool: async () => {
    set({ status: "loading", error: null });
    try {
      const pool = await ammApi.getPoolStatus();
      set({ pool, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load pool status" });
    }
  },
  getQuote: async (payload) => {
    set({ status: "loading", error: null });
    try {
      const quote = await ammApi.quoteExactOutput(payload);
      set({ quote, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to quote" });
    }
  },
  swap: async (payload) => {
    set({ status: "loading", error: null });
    try {
      const quote = await ammApi.swapExactOutput(payload);
      const pool = await ammApi.getPoolStatus();
      set({ quote, pool, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to swap" });
    }
  },
}));

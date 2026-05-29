import axios from "axios";
import { create } from "zustand";
import { ammV2Api } from "../../services/api/amm-v2.api";
import { circuitBreakerStatusApi } from "../../services/api/circuit-breaker-status.api";
import { SWAP_ERROR } from "../../types/amm-v2.types";
import type {
  AMMQuote,
  PoolStatus,
  SwapOrder,
  SwapRequest,
} from "../../types/amm-v2.types";

type AmmV2Store = {
  quote: AMMQuote | null;
  quoteTimestamp: number | null;
  swapResult: SwapOrder | null;
  poolStatus: PoolStatus | null;
  circuitBreakerState: string | null;
  ammApproved: boolean;
  status: "idle" | "loading" | "error";
  error: string | null;
  fetchQuote: (pair: string, amount_out: string) => Promise<void>;
  executeSwap: (payload: SwapRequest) => Promise<void>;
  fetchPoolStatus: (pair: string) => Promise<void>;
  fetchCircuitBreakerState: (pair: string) => Promise<void>;
  approveAmm: (amount: string, side?: "A" | "B") => Promise<void>;
  clearQuote: () => void;
};

function extractApiError(error: unknown, fallback: string): string {
  if (axios.isAxiosError(error)) {
    const apiError = error.response?.data as { error?: string; message?: string } | undefined;
    if (apiError?.error) {
      return apiError.error;
    }
    if (apiError?.message) {
      return apiError.message;
    }
  }
  return error instanceof Error ? error.message : fallback;
}

export const useAmmV2Store = create<AmmV2Store>((set) => ({
  quote: null,
  quoteTimestamp: null,
  swapResult: null,
  poolStatus: null,
  circuitBreakerState: null,
  ammApproved: false,
  status: "idle",
  error: null,
  fetchQuote: async (pair, amount_out) => {
    set({ status: "loading", error: null });
    try {
      const quote = await ammV2Api.getQuote(pair, amount_out);
      set({ quote, quoteTimestamp: Date.now(), ammApproved: false, status: "idle" });
    } catch (error) {
      set({ status: "error", error: extractApiError(error, "Unable to fetch quote") });
    }
  },
  executeSwap: async (payload) => {
    const state = useAmmV2Store.getState();
    if (state.poolStatus?.pool_status && state.poolStatus.pool_status !== "ACTIVE") {
      set({ status: "error", error: SWAP_ERROR.POOL_NOT_ACTIVE });
      return;
    }
    if (state.circuitBreakerState === "HALTED") {
      set({ status: "error", error: SWAP_ERROR.CIRCUIT_BREAKER_HALTED });
      return;
    }
    if (!state.ammApproved) {
      set({ status: "error", error: SWAP_ERROR.APPROVE_AMM_REQUIRED });
      return;
    }

    set({ status: "loading", error: null });
    try {
      const swapResult = await ammV2Api.executeSwap(payload);
      set({ swapResult, ammApproved: false, status: "idle" });
    } catch (error) {
      set({ status: "error", error: extractApiError(error, "Unable to execute swap") });
    }
  },
  fetchPoolStatus: async (pair) => {
    set({ status: "loading", error: null });
    try {
      const poolStatus = await ammV2Api.getPoolStatus(pair);
      set({ poolStatus, status: "idle" });
    } catch (error) {
      set({ status: "error", error: extractApiError(error, "Unable to fetch pool status") });
    }
  },
  fetchCircuitBreakerState: async (pair) => {
    try {
      const status = await circuitBreakerStatusApi.getStatus(pair);
      set({ circuitBreakerState: status.state });
    } catch {
      set({ circuitBreakerState: null });
    }
  },
  approveAmm: async (amount, side) => {
    set({ status: "loading", error: null });
    try {
      await ammV2Api.approveAmm(side ? { amount, side } : { amount });
      set({ status: "idle", ammApproved: true });
    } catch (error) {
      set({ status: "error", error: extractApiError(error, "Unable to approve AMM") });
    }
  },
  clearQuote: () => {
    set({ quote: null, quoteTimestamp: null });
  },
}));

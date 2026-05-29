import { create } from "zustand";
import { governanceApi } from "../services/api";
import type { AsyncStatus, CircuitBreakerPayload, CircuitBreakerState } from "../types";

type CircuitBreakerStore = {
  circuitBreaker: CircuitBreakerState | null;
  status: AsyncStatus;
  error: string | null;
  fetchState: () => Promise<void>;
  toggle: (payload: CircuitBreakerPayload) => Promise<void>;
};

export const useCircuitBreakerStore = create<CircuitBreakerStore>((set) => ({
  circuitBreaker: null,
  status: "idle",
  error: null,
  fetchState: async () => {
    set({ status: "loading", error: null });
    try {
      const circuitBreaker = await governanceApi.getCircuitBreaker();
      set({ circuitBreaker, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load circuit breaker" });
    }
  },
  toggle: async (payload) => {
    set({ status: "loading", error: null });
    try {
      const result = await governanceApi.setCircuitBreaker(payload);
      set({
        circuitBreaker: {
          state: result.state,
          updatedAt: result.updatedAt,
          updatedBy: result.updatedBy,
        },
        status: "idle",
      });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to update circuit breaker" });
    }
  },
}));

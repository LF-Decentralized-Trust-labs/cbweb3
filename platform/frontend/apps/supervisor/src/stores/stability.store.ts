import { create } from "zustand";
import { stabilityApi } from "../services/api";
import type { AMMConfigRequest, CircuitBreakerRequest, PoolStatus, StabilityAlert } from "../types";

type StabilityState = {
  pools: PoolStatus[];
  alerts: StabilityAlert[];
  circuitBreakerActive: boolean;
  status: "idle" | "loading" | "error";
  error: string | null;
  refresh: () => Promise<void>;
  toggleCircuitBreaker: (payload: CircuitBreakerRequest) => Promise<void>;
  updateConfig: (payload: AMMConfigRequest) => Promise<void>;
};

export const useStabilityStore = create<StabilityState>((set, get) => ({
  pools: [],
  alerts: [],
  circuitBreakerActive: false,
  status: "idle",
  error: null,
  refresh: async () => {
    set({ status: "loading", error: null });
    try {
      const [pools, alerts] = await Promise.all([stabilityApi.getPoolStatuses(), stabilityApi.getAlerts()]);
      set({ pools, alerts, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to fetch stability data" });
    }
  },
  toggleCircuitBreaker: async (payload) => {
    set({ status: "loading", error: null });
    try {
      const result = await stabilityApi.setCircuitBreaker(payload);
      set({ circuitBreakerActive: result.isActive, status: "idle" });
      await get().refresh();
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to set circuit breaker" });
    }
  },
  updateConfig: async (payload) => {
    set({ status: "loading", error: null });
    try {
      await stabilityApi.updateAmmConfig(payload);
      set({ status: "idle" });
      await get().refresh();
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to update AMM config" });
    }
  },
}));

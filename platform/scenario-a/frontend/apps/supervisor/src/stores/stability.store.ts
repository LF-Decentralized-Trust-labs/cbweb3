import { create } from "zustand";
import { stabilityApi } from "../services/api";
import type { HTLCSummary, StabilityAlert } from "../types";

type StabilityState = {
  htlcs: HTLCSummary[];
  alerts: StabilityAlert[];
  status: "idle" | "loading" | "error";
  error: string | null;
  refresh: () => Promise<void>;
};

export const useStabilityStore = create<StabilityState>((set) => ({
  htlcs: [],
  alerts: [],
  status: "idle",
  error: null,
  refresh: async () => {
    set({ status: "loading", error: null });
    try {
      const [htlcs, alerts] = await Promise.all([stabilityApi.getHTLCs(), stabilityApi.getAlerts()]);
      set({ htlcs, alerts, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to fetch settlement data" });
    }
  },
}));

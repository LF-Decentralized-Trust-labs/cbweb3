// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { stabilityApi } from "../services/api";
import type { PoolStatus, StabilityAlert } from "../types";

type StabilityState = {
  pools: PoolStatus[];
  alerts: StabilityAlert[];
  status: "idle" | "loading" | "error";
  error: string | null;
  refresh: () => Promise<void>;
};

export const useStabilityStore = create<StabilityState>((set) => ({
  pools: [],
  alerts: [],
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
}));

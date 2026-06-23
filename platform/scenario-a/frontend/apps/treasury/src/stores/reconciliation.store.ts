// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { reconciliationApi } from "../services/api";
import type { AsyncStatus, SpokeHubDelta, TvlSnapshot } from "../types";

type ReconciliationState = {
  tvl: TvlSnapshot[];
  delta: SpokeHubDelta | null;
  status: AsyncStatus;
  error: string | null;
  fetch: () => Promise<void>;
};

export const useReconciliationStore = create<ReconciliationState>((set) => ({
  tvl: [],
  delta: null,
  status: "idle",
  error: null,
  fetch: async () => {
    set({ status: "loading", error: null });
    try {
      const [tvl, delta] = await Promise.all([reconciliationApi.getTvl(), reconciliationApi.getDelta()]);
      set({ tvl, delta, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load reconciliation data" });
    }
  },
}));

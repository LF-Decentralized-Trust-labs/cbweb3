// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { poolApi } from "../services/api";
import type { AsyncStatus, PoolFetchFailure, PoolStatus } from "../types";

type PoolState = {
  pools: PoolStatus[];
  // Pairs the backend could not read. Kept separate from `error` (which means the NOC
  // backend itself was unreachable) so the page can say which of the two happened.
  failures: PoolFetchFailure[];
  status: AsyncStatus;
  error: string | null;
  fetch: () => Promise<void>;
};

export const usePoolStore = create<PoolState>((set) => ({
  pools: [],
  failures: [],
  status: "idle",
  error: null,
  fetch: async () => {
    set({ status: "loading", error: null });
    try {
      const { pools, failures } = await poolApi.list();
      set({ pools, failures, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load pools" });
    }
  },
}));

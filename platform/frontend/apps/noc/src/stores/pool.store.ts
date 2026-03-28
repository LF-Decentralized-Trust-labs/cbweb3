import { create } from "zustand";
import { poolApi } from "../services/api";
import type { AsyncStatus, PoolStatus } from "../types";

type PoolState = {
  pools: PoolStatus[];
  status: AsyncStatus;
  error: string | null;
  fetch: () => Promise<void>;
};

export const usePoolStore = create<PoolState>((set) => ({
  pools: [],
  status: "idle",
  error: null,
  fetch: async () => {
    set({ status: "loading", error: null });
    try {
      const pools = await poolApi.list();
      set({ pools, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load pools" });
    }
  },
}));

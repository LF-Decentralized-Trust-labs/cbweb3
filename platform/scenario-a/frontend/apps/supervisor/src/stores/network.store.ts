import { create } from "zustand";
import { networkApi } from "../services/api";
import type { NetworkOverview } from "../types";

type NetworkState = {
  overview: NetworkOverview | null;
  status: "idle" | "loading" | "error";
  error: string | null;
  refresh: () => Promise<void>;
};

export const useNetworkStore = create<NetworkState>((set) => ({
  overview: null,
  status: "idle",
  error: null,
  refresh: async () => {
    set({ status: "loading", error: null });
    try {
      const overview = await networkApi.getOverview();
      set({ overview, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to fetch overview" });
    }
  },
}));

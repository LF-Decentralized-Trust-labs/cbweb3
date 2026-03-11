import { create } from "zustand";
import { healthApi } from "../services/api";
import type { AsyncStatus, InfrastructureNode } from "../types";

type InfrastructureState = {
  nodes: InfrastructureNode[];
  status: AsyncStatus;
  error: string | null;
  fetch: () => Promise<void>;
};

export const useInfrastructureStore = create<InfrastructureState>((set) => ({
  nodes: [],
  status: "idle",
  error: null,
  fetch: async () => {
    set({ status: "loading", error: null });
    try {
      const nodes = await healthApi.getInfrastructure();
      set({ nodes, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load infrastructure" });
    }
  },
}));

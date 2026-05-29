import { create } from "zustand";
import { healthApi } from "../services/api";
import type { AsyncStatus, TopologyEdge, TopologyNode } from "../types";

type TopologyState = {
  nodes: TopologyNode[];
  edges: TopologyEdge[];
  status: AsyncStatus;
  error: string | null;
  fetch: () => Promise<void>;
};

export const useTopologyStore = create<TopologyState>((set) => ({
  nodes: [],
  edges: [],
  status: "idle",
  error: null,
  fetch: async () => {
    set({ status: "loading", error: null });
    try {
      const topology = await healthApi.getTopology();
      set({ nodes: topology.nodes, edges: topology.edges, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load topology" });
    }
  },
}));

import { create } from "zustand";
import { nocBackendApi } from "../services/api";
import type { AsyncStatus, NocSpoke } from "../types";

type SpokeState = {
  spokes: NocSpoke[];
  selectedSpokeId: string | null;
  status: AsyncStatus;
  error: string | null;
  fetchSpokes: () => Promise<void>;
  selectSpoke: (id: string) => void;
};

export const useSpokeStore = create<SpokeState>((set, get) => ({
  spokes: [],
  selectedSpokeId: null,
  status: "idle",
  error: null,
  fetchSpokes: async () => {
    set({ status: "loading", error: null });
    try {
      const spokes = await nocBackendApi.getSpokes();
      const current = get().selectedSpokeId;
      set({
        spokes,
        status: "idle",
        // Auto-select first spoke if none selected
        selectedSpokeId: current ?? (spokes[0]?.id ?? null),
      });
    } catch (err) {
      set({ status: "error", error: err instanceof Error ? err.message : "Failed to load spokes" });
    }
  },
  selectSpoke: (id) => set({ selectedSpokeId: id }),
}));

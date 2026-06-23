// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { nocBackendApi } from "../services/api";
import type { AsyncStatus, NocSpoke } from "../types";

type SpokeState = {
  spokes: NocSpoke[];
  selectedSpokeId: string | null;
  userSelected: boolean;
  status: AsyncStatus;
  error: string | null;
  fetchSpokes: () => Promise<void>;
  selectSpoke: (id: string | null) => void;
};

export const useSpokeStore = create<SpokeState>((set, get) => ({
  spokes: [],
  selectedSpokeId: null,
  userSelected: false,
  status: "idle",
  error: null,
  fetchSpokes: async () => {
    set({ status: "loading", error: null });
    try {
      const spokes = await nocBackendApi.getSpokes();
      const { selectedSpokeId, userSelected } = get();
      set({
        spokes,
        status: "idle",
        // Auto-select first spoke only if the user hasn't made an explicit choice yet
        selectedSpokeId: userSelected ? selectedSpokeId : (selectedSpokeId ?? (spokes[0]?.id ?? null)),
      });
    } catch (err) {
      set({ status: "error", error: err instanceof Error ? err.message : "Failed to load spokes" });
    }
  },
  selectSpoke: (id) => set({ selectedSpokeId: id, userSelected: true }),
}));

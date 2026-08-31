// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { nocBackendApi } from "../services/api";
import type { AsyncStatus, NocComponent } from "../types";

type ComponentState = {
  components: NocComponent[];
  status: AsyncStatus;
  error: string | null;
  fetchComponents: (spokeId: string) => Promise<void>;
  clearComponents: () => void;
};

export const useInfrastructureStore = create<ComponentState>((set) => ({
  components: [],
  status: "idle",
  error: null,
  clearComponents: () => set({ components: [] }),
  fetchComponents: async (spokeId: string) => {
    set({ status: "loading", error: null });
    try {
      const components = await nocBackendApi.getComponents(spokeId);
      set({ components, status: "idle" });
    } catch (err) {
      set({ status: "error", error: err instanceof Error ? err.message : "Failed to load components" });
    }
  },
}));

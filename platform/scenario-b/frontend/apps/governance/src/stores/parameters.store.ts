// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { governanceApi } from "../services/api";
import type { AsyncStatus, GovernanceParameters, UpdateParametersPayload } from "../types";

type ParametersStore = {
  parameters: GovernanceParameters | null;
  status: AsyncStatus;
  error: string | null;
  fetch: () => Promise<void>;
  update: (payload: UpdateParametersPayload) => Promise<void>;
};

export const useParametersStore = create<ParametersStore>((set) => ({
  parameters: null,
  status: "idle",
  error: null,
  fetch: async () => {
    set({ status: "loading", error: null });
    try {
      const parameters = await governanceApi.getParameters();
      set({ parameters, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load parameters" });
    }
  },
  update: async (payload) => {
    set({ status: "loading", error: null });
    try {
      const parameters = await governanceApi.updateParameters(payload);
      set({ parameters, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to update parameters" });
    }
  },
}));

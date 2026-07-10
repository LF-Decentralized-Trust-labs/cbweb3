// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { identityApi } from "../services/api/identity.api";

type IdentityState = {
  identities: string[];
  status: "idle" | "loading" | "error";
  error: string | null;
  fetchAll: () => Promise<void>;
};

export const useIdentityStore = create<IdentityState>((set) => ({
  identities: [],
  status: "idle",
  error: null,
  fetchAll: async () => {
    set({ status: "loading", error: null });
    try {
      const identities = await identityApi.list();
      set({ identities, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load identities" });
    }
  },
}));

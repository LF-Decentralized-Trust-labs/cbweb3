// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { identityApi } from "../services/api/identity.api";

type IdentityState = {
  identities: string[];
  // Whether the backend reported the roster as configured. Starts true so the
  // form does not flash a "not configured" warning before the first fetch;
  // it is set from the response on load, and forced false on fetch failure.
  configured: boolean;
  status: "idle" | "loading" | "error";
  error: string | null;
  fetchAll: () => Promise<void>;
};

export const useIdentityStore = create<IdentityState>((set) => ({
  identities: [],
  configured: true,
  status: "idle",
  error: null,
  fetchAll: async () => {
    set({ status: "loading", error: null });
    try {
      const { identities, configured } = await identityApi.list();
      set({ identities, configured, status: "idle" });
    } catch (error) {
      set({
        status: "error",
        configured: false,
        identities: [],
        error: error instanceof Error ? error.message : "Unable to load identities",
      });
    }
  },
}));

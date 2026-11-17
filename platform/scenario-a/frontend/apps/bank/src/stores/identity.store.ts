// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { identityApi } from "../services/api/identity.api";
import { apiErrorMessage } from "@cbweb3/ui";

type IdentityState = {
  identities: string[];
  // Whether the backend reported the roster as configured. Starts true so the
  // form does not flash a "not configured" warning before the first fetch;
  // it is set from the response on load, and forced false on fetch failure.
  configured: boolean;
  status: "idle" | "loading" | "error";
  // True once a fetch has succeeded. "idle" alone cannot tell a loaded roster from one that
  // has not been requested yet, and a form that derives parties from the roster must not treat
  // "not loaded yet" as "not in the roster".
  loaded: boolean;
  error: string | null;
  fetchAll: () => Promise<void>;
};

export const useIdentityStore = create<IdentityState>((set) => ({
  identities: [],
  configured: true,
  status: "idle",
  loaded: false,
  error: null,
  fetchAll: async () => {
    set({ status: "loading", error: null });
    try {
      const { identities, configured } = await identityApi.list();
      set({ identities, configured, status: "idle", loaded: true });
    } catch (error) {
      set({
        status: "error",
        configured: false,
        identities: [],
        error: apiErrorMessage(error, "Unable to load identities"),
      });
    }
  },
}));

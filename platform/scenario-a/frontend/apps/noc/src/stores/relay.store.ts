import { create } from "zustand";
import { relayApi } from "../services/api";
import type { AsyncStatus, RelayStatus } from "../types";

type RelayState = {
  relays: RelayStatus[];
  status: AsyncStatus;
  error: string | null;
  fetch: () => Promise<void>;
};

export const useRelayStore = create<RelayState>((set) => ({
  relays: [],
  status: "idle",
  error: null,
  fetch: async () => {
    set({ status: "loading", error: null });
    try {
      const relays = await relayApi.list();
      set({ relays, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load relays" });
    }
  },
}));

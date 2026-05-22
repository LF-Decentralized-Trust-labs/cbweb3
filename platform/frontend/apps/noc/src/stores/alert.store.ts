import { create } from "zustand";
import { nocBackendApi } from "../services/api";
import type { AsyncStatus, NocAlert } from "../types";

type AlertState = {
  alerts: NocAlert[];
  status: AsyncStatus;
  error: string | null;
  fetchAlerts: (spokeId?: string) => Promise<void>;
};

export const useAlertStore = create<AlertState>((set) => ({
  alerts: [],
  status: "idle",
  error: null,
  fetchAlerts: async (spokeId?: string) => {
    set({ status: "loading", error: null });
    try {
      const alerts = await nocBackendApi.getAlerts(spokeId);
      set({ alerts, status: "idle" });
    } catch (err) {
      set({ status: "error", error: err instanceof Error ? err.message : "Failed to load alerts" });
    }
  },
}));

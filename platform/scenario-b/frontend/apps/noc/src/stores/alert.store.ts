import { create } from "zustand";
import { nocBackendApi } from "../services/api";
import type { AsyncStatus, NocAlert, NocAlertDetail } from "../types";

type AlertState = {
  alerts: NocAlert[];
  resolvedAlerts: NocAlert[];
  status: AsyncStatus;
  error: string | null;
  selectedAlert: NocAlertDetail | null;
  selectedAlertStatus: AsyncStatus;
  fetchAlerts: (spokeId?: string) => Promise<void>;
  fetchResolvedAlerts: (spokeId?: string) => Promise<void>;
  selectAlert: (id: string) => Promise<void>;
  clearSelectedAlert: () => void;
  acknowledgeAlert: (id: string) => Promise<void>;
  dismissAlert: (id: string) => Promise<void>;
};

export const useAlertStore = create<AlertState>((set, get) => ({
  alerts: [],
  resolvedAlerts: [],
  status: "idle",
  error: null,
  selectedAlert: null,
  selectedAlertStatus: "idle",

  fetchAlerts: async (spokeId?: string) => {
    set({ status: "loading", error: null });
    try {
      const alerts = await nocBackendApi.getAlerts(spokeId);
      set({ alerts, status: "idle" });
    } catch (err) {
      set({ status: "error", error: err instanceof Error ? err.message : "Failed to load alerts" });
    }
  },

  fetchResolvedAlerts: async (spokeId?: string) => {
    try {
      const resolvedAlerts = await nocBackendApi.getAlerts(spokeId, "RESOLVED");
      set({ resolvedAlerts });
    } catch (err) {
      set({ error: err instanceof Error ? err.message : "Failed to load resolved alerts" });
    }
  },

  selectAlert: async (id: string) => {
    set({ selectedAlertStatus: "loading" });
    try {
      const detail = await nocBackendApi.getAlert(id);
      set({ selectedAlert: detail, selectedAlertStatus: "idle" });
    } catch (err) {
      set({ selectedAlertStatus: "error", error: err instanceof Error ? err.message : "Failed to load alert" });
    }
  },

  clearSelectedAlert: () => set({ selectedAlert: null, selectedAlertStatus: "idle" }),

  acknowledgeAlert: async (id: string) => {
    await nocBackendApi.acknowledgeAlert(id);
    const current = get().selectedAlert;
    if (current?.id === id) {
      await get().selectAlert(id);
    }
    set((s) => ({
      alerts: s.alerts.map((a) =>
        a.id === id ? { ...a, acknowledged_by: "me" } : a,
      ),
    }));
  },

  dismissAlert: async (id: string) => {
    await nocBackendApi.dismissAlert(id);
    set((s) => ({
      alerts: s.alerts.filter((a) => a.id !== id),
      selectedAlert: s.selectedAlert?.id === id ? null : s.selectedAlert,
    }));
  },
}));

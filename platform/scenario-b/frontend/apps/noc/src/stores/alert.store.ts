import { create } from "zustand";
import type { NocAlert } from "../types";

type AlertState = {
  alerts: NocAlert[];
  acknowledged: string[];
  addAlert: (alert: NocAlert) => void;
  acknowledge: (alertId: string) => void;
  clearAll: () => void;
};

export const useAlertStore = create<AlertState>((set, get) => ({
  alerts: [],
  acknowledged: [],
  addAlert: (alert) => {
    const exists = get().alerts.some((item) => item.message === alert.message && item.severity === alert.severity);
    if (exists) {
      return;
    }
    set((state) => ({ alerts: [alert, ...state.alerts].slice(0, 80) }));
  },
  acknowledge: (alertId) => set((state) => ({ acknowledged: [alertId, ...state.acknowledged] })),
  clearAll: () => set({ alerts: [], acknowledged: [] }),
}));

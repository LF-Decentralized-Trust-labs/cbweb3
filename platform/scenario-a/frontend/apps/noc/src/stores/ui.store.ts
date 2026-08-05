// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { persist } from "zustand/middleware";

type UiState = {
  muteAlerts: boolean;
  timezone: string;
  fallbackPollingSeconds: number;
  alertEmail: string;
  setMuteAlerts: (value: boolean) => void;
  setTimezone: (value: string) => void;
  setFallbackPollingSeconds: (value: number) => void;
  setAlertEmail: (value: string) => void;
};

export const useUiStore = create<UiState>()(
  persist(
    (set) => ({
      muteAlerts: false,
      timezone: "UTC",
      fallbackPollingSeconds: 15,
      alertEmail: "noc-ops@cbweb3.local",
      setMuteAlerts: (value) => set({ muteAlerts: value }),
      setTimezone: (value) => set({ timezone: value }),
      setFallbackPollingSeconds: (value) => set({ fallbackPollingSeconds: value }),
      setAlertEmail: (value) => set({ alertEmail: value }),
    }),
    { name: "noc-ui-settings" },
  ),
);

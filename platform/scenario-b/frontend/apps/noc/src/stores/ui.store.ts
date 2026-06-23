// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";

type UiState = {
  muteAlerts: boolean;
  timezone: string;
  fallbackPollingSeconds: number;
  setMuteAlerts: (value: boolean) => void;
  setTimezone: (value: string) => void;
  setFallbackPollingSeconds: (value: number) => void;
};

export const useUiStore = create<UiState>((set) => ({
  muteAlerts: false,
  timezone: "UTC",
  fallbackPollingSeconds: 15,
  setMuteAlerts: (value) => set({ muteAlerts: value }),
  setTimezone: (value) => set({ timezone: value }),
  setFallbackPollingSeconds: (value) => set({ fallbackPollingSeconds: value }),
}));

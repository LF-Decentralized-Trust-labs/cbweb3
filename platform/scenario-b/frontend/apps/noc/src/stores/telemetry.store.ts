// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import type { AsyncStatus, TelemetryFrame } from "../types";

type TelemetryState = {
  frames: TelemetryFrame[];
  status: AsyncStatus;
  error: string | null;
  fetchSnapshot: () => Promise<void>;
  pushFrame: (frame: TelemetryFrame) => void;
  clear: () => void;
};

export const useTelemetryStore = create<TelemetryState>((set) => ({
  frames: [],
  status: "idle",
  error: null,
  // Telemetry is pushed via WebSocket; this is a no-op kept for interface compatibility.
  fetchSnapshot: async () => { set({ status: "idle" }); },
  pushFrame: (frame) => set((state) => ({ frames: [frame, ...state.frames].slice(0, 80) })),
  clear: () => set({ frames: [] }),
}));

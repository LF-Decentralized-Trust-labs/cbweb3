import { create } from "zustand";
import { healthApi } from "../services/api";
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
  fetchSnapshot: async () => {
    set({ status: "loading", error: null });
    try {
      const frames = await healthApi.getTelemetrySnapshot();
      set({ frames, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to fetch telemetry" });
    }
  },
  pushFrame: (frame) => set((state) => ({ frames: [frame, ...state.frames].slice(0, 80) })),
  clear: () => set({ frames: [] }),
}));

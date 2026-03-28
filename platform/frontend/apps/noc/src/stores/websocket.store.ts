import { create } from "zustand";
import { telemetryService } from "../services/websocket/telemetry.service";
import { useAlertStore } from "./alert.store";
import { useTelemetryStore } from "./telemetry.store";

type WebsocketState = {
  isConnected: boolean;
  stale: boolean;
  reconnectAttempts: number;
  connect: () => void;
  disconnect: () => void;
};

let stopStream: (() => void) | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

export const useWebsocketStore = create<WebsocketState>((set, get) => ({
  isConnected: false,
  stale: true,
  reconnectAttempts: 0,
  connect: () => {
    if (stopStream) {
      return;
    }

    const onDrop = () => {
      if (stopStream) {
        stopStream();
        stopStream = null;
      }

      const attempts = get().reconnectAttempts + 1;
      const backoffMs = Math.min(1000 * 2 ** attempts, 15_000);
      set({ isConnected: false, stale: true, reconnectAttempts: attempts });

      reconnectTimer = setTimeout(() => {
        reconnectTimer = null;
        get().connect();
      }, backoffMs);
    };

    stopStream = telemetryService.start({
      onFrame: (frame) => {
        useTelemetryStore.getState().pushFrame(frame);
        set({ stale: false, isConnected: true, reconnectAttempts: 0 });

        if (!frame.healthy || frame.cpuPct > 90 || frame.memoryPct > 90) {
          useAlertStore.getState().addAlert({
            id: `alert_${frame.id}`,
            source: "TELEMETRY",
            severity: frame.cpuPct > 90 || frame.memoryPct > 90 ? "CRITICAL" : "WARNING",
            message: `${frame.component} ${frame.componentId} degraded metrics`,
            correlationId: frame.id,
            createdAt: frame.timestamp,
          });
        }
      },
      onDrop,
    });

    set({ isConnected: true, stale: false });
  },
  disconnect: () => {
    if (stopStream) {
      stopStream();
      stopStream = null;
    }
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
    set({ isConnected: false, stale: true, reconnectAttempts: 0 });
  },
}));

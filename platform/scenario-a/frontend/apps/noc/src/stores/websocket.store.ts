import { create } from "zustand";

type WebsocketState = {
  isConnected: boolean;
  stale: boolean;
  reconnectAttempts: number;
  connect: () => void;
  disconnect: () => void;
};

export const useWebsocketStore = create<WebsocketState>(() => ({
  isConnected: false,
  stale: true,
  reconnectAttempts: 0,
  connect: () => { /* telemetry stream not yet implemented */ },
  disconnect: () => { /* no-op */ },
}));

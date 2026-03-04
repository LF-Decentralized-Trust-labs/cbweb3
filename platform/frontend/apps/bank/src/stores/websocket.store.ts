import { create } from "zustand";
import { eventsService } from "../services/websocket/events.service";
import type { BankEvent } from "../types";

type WebSocketState = {
  connected: boolean;
  events: BankEvent[];
  connect: () => void;
  disconnect: () => void;
};

export const useWebsocketStore = create<WebSocketState>((set, get) => ({
  connected: false,
  events: [],
  connect: () => {
    if (get().connected) return;
    eventsService.connect();
    eventsService.subscribe((event) => {
      set((state) => ({ events: [event, ...state.events].slice(0, 50) }));
    });
    set({ connected: true });
  },
  disconnect: () => {
    eventsService.disconnect();
    set({ connected: false });
  },
}));

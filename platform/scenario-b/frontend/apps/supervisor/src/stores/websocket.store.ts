// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { sseService } from "../services/websocket/sse.service";
import type { SupervisorEvent } from "../types";

type WebsocketState = {
  connected: boolean;
  events: SupervisorEvent[];
  connect: () => void;
  disconnect: () => void;
};

export const useWebsocketStore = create<WebsocketState>((set, get) => ({
  connected: false,
  events: [],
  connect: () => {
    if (get().connected) return;
    sseService.connect();
    sseService.subscribe((event) => {
      set((state) => ({ events: [event, ...state.events].slice(0, 50) }));
    });
    set({ connected: true });
  },
  disconnect: () => {
    sseService.disconnect();
    set({ connected: false });
  },
}));

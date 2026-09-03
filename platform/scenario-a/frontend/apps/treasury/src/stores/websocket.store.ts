// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { treasuryEventsService } from "../services/websocket/events.service";
import type { TreasuryEvent } from "../types";

type WebsocketState = {
  isConnected: boolean;
  events: TreasuryEvent[];
  connect: () => void;
  disconnect: () => void;
};

let stop: (() => void) | null = null;

export const useWebsocketStore = create<WebsocketState>((set) => ({
  isConnected: false,
  events: [],
  connect: () => {
    if (stop) {
      return;
    }
    stop = treasuryEventsService.start((event) => {
      set((state) => ({ events: [event, ...state.events].slice(0, 30) }));
    });
    set({ isConnected: true });
  },
  disconnect: () => {
    if (stop) {
      stop();
      stop = null;
    }
    set({ isConnected: false });
  },
}));

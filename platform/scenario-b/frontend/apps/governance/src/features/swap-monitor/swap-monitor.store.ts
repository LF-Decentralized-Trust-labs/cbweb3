// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { httpClientV2 } from "../../services/api/http-client";

type SwapStatus =
  | "PENDING"
  | "BRIDGE_IN_PROGRESS"
  | "SWAP_IN_PROGRESS"
  | "BRIDGE_OUT_PROGRESS"
  | "COMPLETED"
  | "BRIDGE_OUT_FAILED";

export interface SwapMonitorEntry {
  swap_id: string;
  payer_bank_id: string;
  beneficiary_bank_id: string;
  status: SwapStatus;
  bridge_in_position_id?: string;
  swap_tx_ref?: string;
  bridge_out_position_id?: string;
  created_at?: string;
  updated_at?: string;
}

type FetchState = "loading" | "error" | "done";

type SwapMonitorStore = {
  swapIds: string[];
  swapRecords: Record<string, SwapMonitorEntry>;
  fetchStatus: Record<string, FetchState>;
  fetchSwapRecord: (swapId: string) => Promise<void>;
  addSwapId: (swapId: string) => void;
};

export const useSwapMonitorStore = create<SwapMonitorStore>((set, get) => ({
  swapIds: [],
  swapRecords: {},
  fetchStatus: {},
  fetchSwapRecord: async (swapId) => {
    if (!swapId.trim()) {
      return;
    }

    set((state) => ({
      fetchStatus: {
        ...state.fetchStatus,
        [swapId]: "loading",
      },
    }));

    try {
      const response = await httpClientV2.get<SwapMonitorEntry>(`/amm/swap/cross-currency/${swapId}`);
      set((state) => ({
        swapRecords: {
          ...state.swapRecords,
          [swapId]: response.data,
        },
        fetchStatus: {
          ...state.fetchStatus,
          [swapId]: "done",
        },
      }));
    } catch {
      set((state) => ({
        fetchStatus: {
          ...state.fetchStatus,
          [swapId]: "error",
        },
      }));
    }
  },
  addSwapId: (swapId) => {
    const normalized = swapId.trim();
    if (!normalized) {
      return;
    }

    const current = get().swapIds;
    if (current.includes(normalized)) {
      return;
    }

    set({ swapIds: [...current, normalized] });
  },
}));

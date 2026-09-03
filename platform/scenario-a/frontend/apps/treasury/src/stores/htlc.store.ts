// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { htlcApi } from "../services/api";
import type { HTLCLock, SearchHTLCParams } from "../types";

type HtlcMonitorState = {
  locks: HTLCLock[];
  total: number;
  status: "idle" | "loading" | "error";
  error: string | null;
  fetch: (params?: SearchHTLCParams) => Promise<void>;
  getDetail: (contractId: string) => Promise<HTLCLock>;
};

export const useHtlcMonitorStore = create<HtlcMonitorState>((set) => ({
  locks: [],
  total: 0,
  status: "idle",
  error: null,
  fetch: async (params = {}) => {
    set({ status: "loading", error: null });
    try {
      const response = await htlcApi.search(params);
      set({ locks: response.locks, total: response.total, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load HTLC monitor data" });
    }
  },
  getDetail: async (contractId) => {
    return htlcApi.getStatus(contractId);
  },
}));

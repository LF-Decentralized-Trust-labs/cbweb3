// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { fundingApi } from "../services/api";
import type { AsyncStatus, FundingRequest } from "../types";

type FundingState = {
  requests: FundingRequest[];
  status: AsyncStatus;
  error: string | null;
  fetchRequests: () => Promise<void>;
  approveRequest: (requestId: string, reason?: string) => Promise<void>;
  rejectRequest: (requestId: string, reason: string) => Promise<void>;
};

export const useFundingStore = create<FundingState>((set) => ({
  requests: [],
  status: "idle",
  error: null,
  fetchRequests: async () => {
    set({ status: "loading", error: null });
    try {
      const requests = await fundingApi.list();
      set({ requests, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load requests" });
    }
  },
  approveRequest: async (requestId, reason) => {
    set({ status: "loading", error: null });
    try {
      await fundingApi.approve({ requestId, reason });
      const requests = await fundingApi.list();
      set({ requests, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to approve request" });
    }
  },
  rejectRequest: async (requestId, reason) => {
    set({ status: "loading", error: null });
    try {
      await fundingApi.reject({ requestId, reason });
      const requests = await fundingApi.list();
      set({ requests, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to reject request" });
    }
  },
}));

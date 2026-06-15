import { create } from "zustand";
import { transferLimitsApi } from "../services/api";
import type { AsyncStatus, CreateTransferLimitPayload, TransferLimit } from "../types";

type TransferLimitsState = {
  limits: TransferLimit[];
  status: AsyncStatus;
  error: string | null;
  fetch: () => Promise<void>;
  create: (payload: CreateTransferLimitPayload) => Promise<void>;
  remove: (limitId: string) => Promise<void>;
};

export const useTransferLimitsStore = create<TransferLimitsState>((set) => ({
  limits: [],
  status: "idle",
  error: null,

  fetch: async () => {
    set({ status: "loading", error: null });
    try {
      const limits = await transferLimitsApi.list();
      set({ limits, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load transfer limits" });
    }
  },

  create: async (payload) => {
    set({ status: "loading", error: null });
    try {
      await transferLimitsApi.create(payload);
      const limits = await transferLimitsApi.list();
      set({ limits, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to create transfer limit" });
      throw error;
    }
  },

  remove: async (limitId) => {
    set({ status: "loading", error: null });
    try {
      await transferLimitsApi.remove(limitId);
      const limits = await transferLimitsApi.list();
      set({ limits, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to remove transfer limit" });
    }
  },
}));

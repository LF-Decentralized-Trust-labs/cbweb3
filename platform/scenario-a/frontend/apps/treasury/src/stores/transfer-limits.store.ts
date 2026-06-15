import { create } from "zustand";
import { transferLimitsApi } from "../services/api";
import type { CreateTransferLimitRequest, TransferLimit } from "../types";

type TransferLimitsState = {
  limits: TransferLimit[];
  status: "idle" | "loading" | "error";
  error: string | null;
  fetch: () => Promise<void>;
  create: (req: CreateTransferLimitRequest) => Promise<void>;
  remove: (limitId: string) => Promise<void>;
};

export const useTransferLimitsStore = create<TransferLimitsState>((set, get) => ({
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

  create: async (req) => {
    await transferLimitsApi.create(req);
    await get().fetch();
  },

  remove: async (limitId) => {
    await transferLimitsApi.delete(limitId);
    set((state) => ({ limits: state.limits.filter((l) => l.limit_id !== limitId) }));
  },
}));

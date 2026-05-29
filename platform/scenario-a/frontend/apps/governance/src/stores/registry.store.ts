import { create } from "zustand";
import { registryApi } from "../services/api";
import type { ApproveKycPayload, ApproveKycResponse, AsyncStatus, KycStatusEntry, Participant } from "../types";

type RegistryState = {
  participants: Participant[];
  pendingKyc: KycStatusEntry[];
  status: AsyncStatus;
  kycStatus: AsyncStatus;
  error: string | null;
  fetch: () => Promise<void>;
  fetchPendingKyc: () => Promise<void>;
  approveKyc: (payload: ApproveKycPayload) => Promise<ApproveKycResponse | null>;
};

export const useRegistryStore = create<RegistryState>((set, get) => ({
  participants: [],
  pendingKyc: [],
  status: "idle",
  kycStatus: "idle",
  error: null,
  fetch: async () => {
    set({ status: "loading", error: null });
    try {
      const [participants, pendingKyc] = await Promise.all([registryApi.list(), registryApi.listPendingKyc()]);
      set({ participants, pendingKyc, status: "idle", kycStatus: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load registry" });
    }
  },
  fetchPendingKyc: async () => {
    set({ kycStatus: "loading", error: null });
    try {
      const pendingKyc = await registryApi.listPendingKyc();
      set({ pendingKyc, kycStatus: "idle" });
    } catch (error) {
      set({ kycStatus: "error", error: error instanceof Error ? error.message : "Unable to load pending KYC" });
    }
  },
  approveKyc: async (payload) => {
    set({ kycStatus: "loading", error: null });
    try {
      const response = await registryApi.approveKyc(payload);
      await get().fetchPendingKyc();
      set({ kycStatus: "idle" });
      return response;
    } catch (error) {
      set({ kycStatus: "error", error: error instanceof Error ? error.message : "Unable to approve KYC" });
      return null;
    }
  },
}));

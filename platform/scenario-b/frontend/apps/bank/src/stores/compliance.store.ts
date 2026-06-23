// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { complianceApi } from "../services/api";
import type { ComplianceCredential } from "../types";

type ComplianceState = {
  credentials: ComplianceCredential[];
  status: "idle" | "loading" | "error";
  error: string | null;
  fetch: () => Promise<void>;
  attach: (transactionId: string, credentialIds: string[]) => Promise<void>;
};

export const useComplianceStore = create<ComplianceState>((set) => ({
  credentials: [],
  status: "idle",
  error: null,
  fetch: async () => {
    set({ status: "loading", error: null });
    try {
      const credentials = await complianceApi.getCredentials();
      set({ credentials, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load credentials" });
    }
  },
  attach: async (transactionId, credentialIds) => {
    set({ status: "loading", error: null });
    try {
      await complianceApi.attachCredentials(transactionId, credentialIds);
      set({ status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to attach credentials" });
    }
  },
}));

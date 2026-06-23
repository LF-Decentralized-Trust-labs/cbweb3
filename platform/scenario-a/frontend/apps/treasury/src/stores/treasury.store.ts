// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { treasuryApi } from "../services/api";
import type { AsyncStatus, BurnPayload, BurnToMintValidation, MintPayload, SupplySnapshot, TreasuryOperation } from "../types";

type TreasuryState = {
  supply: SupplySnapshot | null;
  operations: TreasuryOperation[];
  validation: BurnToMintValidation | null;
  status: AsyncStatus;
  error: string | null;
  fetch: () => Promise<void>;
  validateBurnToMint: (requestId: string, amount: string) => Promise<void>;
  mint: (payload: MintPayload) => Promise<void>;
  burn: (payload: BurnPayload) => Promise<void>;
};

export const useTreasuryStore = create<TreasuryState>((set) => ({
  supply: null,
  operations: [],
  validation: null,
  status: "idle",
  error: null,
  fetch: async () => {
    set({ status: "loading", error: null });
    try {
      const [supply, operations] = await Promise.all([treasuryApi.getSupply(), treasuryApi.getOperations()]);
      set({ supply, operations, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load treasury data" });
    }
  },
  validateBurnToMint: async (requestId, amount) => {
    set({ status: "loading", error: null });
    try {
      const validation = await treasuryApi.validateBurnToMint(requestId, amount);
      set({ validation, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to validate" });
    }
  },
  mint: async (payload) => {
    set({ status: "loading", error: null });
    try {
      await treasuryApi.mint(payload);
      const [supply, operations] = await Promise.all([treasuryApi.getSupply(), treasuryApi.getOperations()]);
      set({ supply, operations, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to mint" });
    }
  },
  burn: async (payload) => {
    set({ status: "loading", error: null });
    try {
      await treasuryApi.burn(payload);
      const [supply, operations] = await Promise.all([treasuryApi.getSupply(), treasuryApi.getOperations()]);
      set({ supply, operations, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to burn" });
    }
  },
}));

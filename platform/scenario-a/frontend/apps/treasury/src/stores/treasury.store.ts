// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { treasuryApi } from "../services/api";
import type { AsyncStatus, BurnPayload, MintPayload, SupplySnapshot, TreasuryOperation } from "../types";

type TreasuryState = {
  supply: SupplySnapshot | null;
  operations: TreasuryOperation[];
  status: AsyncStatus;
  error: string | null;
  fetch: () => Promise<void>;
  /**
   * Returns null when the operation was accepted, or the failure message to show.
   *
   * These used to return void and swallow the error into `error`, so the pages ran
   * `await mint(...)` and then reported success unconditionally — a rejected burn
   * told the operator "Burn operation submitted" in green (finding R2-M-8 review).
   * Reading `error` from the hook instead would give the value captured at the last
   * render, not the one this call just set, so the outcome is returned directly.
   */
  mint: (payload: MintPayload) => Promise<string | null>;
  burn: (payload: BurnPayload) => Promise<string | null>;
};

export const useTreasuryStore = create<TreasuryState>((set) => ({
  supply: null,
  operations: [],
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
  mint: async (payload) => {
    set({ status: "loading", error: null });
    try {
      await treasuryApi.mint(payload);
    } catch (error) {
      const message = error instanceof Error ? error.message : "Unable to mint";
      set({ status: "error", error: message });
      return message;
    }
    // The operation went through. A refresh failure from here on must NOT be reported
    // as a failed mint: the money has already moved, and telling the operator otherwise
    // invites a retry that would mint a second time.
    try {
      const [supply, operations] = await Promise.all([treasuryApi.getSupply(), treasuryApi.getOperations()]);
      set({ supply, operations, status: "idle" });
    } catch (error) {
      set({
        status: "idle",
        error: `The mint was accepted, but the treasury view could not be refreshed: ${
          error instanceof Error ? error.message : "unknown error"
        }`,
      });
    }
    return null;
  },
  burn: async (payload) => {
    set({ status: "loading", error: null });
    try {
      await treasuryApi.burn(payload);
    } catch (error) {
      const message = error instanceof Error ? error.message : "Unable to burn";
      set({ status: "error", error: message });
      return message;
    }
    // The operation went through. A refresh failure from here on must NOT be reported
    // as a failed burn: the money has already moved, and telling the operator otherwise
    // invites a retry that would burn a second time.
    try {
      const [supply, operations] = await Promise.all([treasuryApi.getSupply(), treasuryApi.getOperations()]);
      set({ supply, operations, status: "idle" });
    } catch (error) {
      set({
        status: "idle",
        error: `The burn was accepted, but the treasury view could not be refreshed: ${
          error instanceof Error ? error.message : "unknown error"
        }`,
      });
    }
    return null;
  },
}));

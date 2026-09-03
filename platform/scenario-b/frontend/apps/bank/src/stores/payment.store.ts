// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import type { AsyncStatus, DepositRecord, EscrowRecord, RedeemRecord } from "../types";
import { paymentApi } from "../services/api";
import { useAuthStore } from "./auth.store";

type PaymentState = {
  deposits: DepositRecord[];
  escrows: EscrowRecord[];
  redeems: RedeemRecord[];
  balance: string | null;
  fiatBalance: string | null;
  tCeBMDecimals: number | null;
  tCeBMSymbol: string | null;
  fCeBMDecimals: number | null;
  fCeBMSymbol: string | null;
  status: AsyncStatus;
  error: string | null;
  fetchAll: () => Promise<void>;
  registerDeposit: (amount: string) => Promise<string>;
  requestEscrow: (depositId: string, amount: string) => Promise<string>;
  requestRedeem: (amount: string) => Promise<string>;
};

const getErrorMessage = (error: unknown, fallback: string) => (error instanceof Error ? error.message : fallback);

function withRequesterContext<T extends Record<string, unknown> & { requester_besu_address?: string }>(
  payload: T,
): T {
  const walletAddress = useAuthStore.getState().profile?.wallet?.trim() ?? "";

  if (walletAddress) {
    payload.requester_besu_address = walletAddress;
  }

  return payload;
}

export const usePaymentStore = create<PaymentState>((set, get) => ({
  deposits: [],
  escrows: [],
  redeems: [],
  balance: null,
  fiatBalance: null,
  tCeBMDecimals: null,
  tCeBMSymbol: null,
  fCeBMDecimals: null,
  fCeBMSymbol: null,
  status: "idle",
  error: null,
  fetchAll: async () => {
    set({ status: "loading", error: null });
    try {
      const [depositsResponse, escrowsResponse, redeemsResponse, balanceResponse, fiatBalanceResponse] =
        await Promise.allSettled([
          paymentApi.listDeposits(),
          paymentApi.listEscrows(),
          paymentApi.listRedeems(),
          paymentApi.getBalance(),
          paymentApi.getFiatBalance(),
        ]);

      set({
        deposits: depositsResponse.status === "fulfilled" ? depositsResponse.value.deposits : [],
        escrows: escrowsResponse.status === "fulfilled" ? escrowsResponse.value.escrows : [],
        redeems: redeemsResponse.status === "fulfilled" ? redeemsResponse.value.redeems : [],
        balance: balanceResponse.status === "fulfilled" ? balanceResponse.value.balance : null,
        tCeBMDecimals: balanceResponse.status === "fulfilled" ? (balanceResponse.value.decimals ?? null) : null,
        tCeBMSymbol: balanceResponse.status === "fulfilled" ? (balanceResponse.value.symbol ?? null) : null,
        fiatBalance: fiatBalanceResponse.status === "fulfilled" ? fiatBalanceResponse.value.balance : null,
        fCeBMDecimals: fiatBalanceResponse.status === "fulfilled" ? (fiatBalanceResponse.value.decimals ?? null) : null,
        fCeBMSymbol: fiatBalanceResponse.status === "fulfilled" ? (fiatBalanceResponse.value.symbol ?? null) : null,
        status: "idle",
      });
    } catch (error) {
      set({ status: "error", error: getErrorMessage(error, "Unable to load payment data") });
    }
  },
  registerDeposit: async (amount) => {
    set({ status: "loading", error: null });
    try {
      const response = await paymentApi.registerDeposit(withRequesterContext({ amount }));
      await get().fetchAll();
      set({ status: "idle" });
      return response.deposit_id;
    } catch (error) {
      const message = getErrorMessage(error, "Unable to register deposit");
      set({ status: "error", error: message });
      throw new Error(message);
    }
  },
  requestEscrow: async (depositId, amount) => {
    set({ status: "loading", error: null });
    try {
      const response = await paymentApi.requestEscrow({ deposit_id: depositId, amount });
      await get().fetchAll();
      set({ status: "idle" });
      return response.escrow_id;
    } catch (error) {
      const message = getErrorMessage(error, "Unable to request escrow");
      set({ status: "error", error: message });
      throw new Error(message);
    }
  },
  requestRedeem: async (amount) => {
    set({ status: "loading", error: null });
    try {
      const response = await paymentApi.requestRedeem(withRequesterContext({ amount }));
      await get().fetchAll();
      set({ status: "idle" });
      return response.redeem_id;
    } catch (error) {
      const message = getErrorMessage(error, "Unable to request redeem");
      set({ status: "error", error: message });
      throw new Error(message);
    }
  },
}));

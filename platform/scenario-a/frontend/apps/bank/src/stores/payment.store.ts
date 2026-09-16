// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import type { AsyncStatus, DepositRecord, EscrowRecord, RedeemRecord } from "../types";
import { paymentApi } from "../services/api";
import { apiErrorMessage } from "@cbweb3/ui";

type PaymentState = {
  deposits: DepositRecord[];
  escrows: EscrowRecord[];
  redeems: RedeemRecord[];
  balance: string | null;
  fiatBalance: string | null;
  // Scales for the two balances above. A balance is unreadable without its scale, so
  // these travel with it from the gateway (ADR-009). The fallbacks are the application
  // scale — hundredths, bounded by the Zeto lock circuit — used only before the first
  // fetch resolves.
  tCeBMDecimals: number;
  tCeBMSymbol: string;
  fiatDecimals: number;
  fiatSymbol: string;
  status: AsyncStatus;
  error: string | null;
  fetchAll: () => Promise<void>;
  registerDeposit: (amount: string) => Promise<string>;
  requestEscrow: (amount: string) => Promise<string>;
  requestRedeem: (amount: string) => Promise<string>;
};

export const usePaymentStore = create<PaymentState>((set, get) => ({
  deposits: [],
  escrows: [],
  redeems: [],
  balance: null,
  fiatBalance: null,
  tCeBMDecimals: 2,
  tCeBMSymbol: "tCeBM",
  fiatDecimals: 2,
  fiatSymbol: "",
  status: "idle",
  error: null,
  fetchAll: async () => {
    set({ status: "loading", error: null });
    try {
      const [depositsResponse, escrowsResponse, redeemsResponse, balanceResponse, fiatBalanceResponse] = await Promise.all([
        paymentApi.listDeposits(),
        paymentApi.listEscrows(),
        paymentApi.listRedeems(),
        paymentApi.getBalance(),
        paymentApi.getFiatBalance(),
      ]);

      set({
        deposits: depositsResponse.deposits,
        escrows: escrowsResponse.escrows,
        redeems: redeemsResponse.redeems,
        balance: balanceResponse.balance,
        fiatBalance: fiatBalanceResponse.balance,
        tCeBMDecimals: balanceResponse.decimals,
        tCeBMSymbol: balanceResponse.symbol || "tCeBM",
        fiatDecimals: fiatBalanceResponse.decimals,
        fiatSymbol: fiatBalanceResponse.symbol,
        status: "idle",
      });
    } catch (error) {
      set({ status: "error", error: apiErrorMessage(error, "Unable to load payment data") });
    }
  },
  registerDeposit: async (amount) => {
    set({ status: "loading", error: null });
    try {
      const response = await paymentApi.registerDeposit({ amount });
      await get().fetchAll();
      set({ status: "idle" });
      return response.deposit_id;
    } catch (error) {
      const message = apiErrorMessage(error, "Unable to register deposit");
      set({ status: "error", error: message });
      throw new Error(message);
    }
  },
  requestEscrow: async (amount) => {
    set({ status: "loading", error: null });
    try {
      const response = await paymentApi.requestEscrow({ amount });
      await get().fetchAll();
      set({ status: "idle" });
      return response.escrow_id;
    } catch (error) {
      const message = apiErrorMessage(error, "Unable to request escrow");
      set({ status: "error", error: message });
      throw new Error(message);
    }
  },
  requestRedeem: async (amount) => {
    set({ status: "loading", error: null });
    try {
      const response = await paymentApi.requestRedeem({ amount });
      await get().fetchAll();
      set({ status: "idle" });
      return response.redeem_id;
    } catch (error) {
      const message = apiErrorMessage(error, "Unable to request redeem");
      set({ status: "error", error: message });
      throw new Error(message);
    }
  },
}));

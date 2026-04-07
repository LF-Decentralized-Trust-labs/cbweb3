import { create } from "zustand";
import type { AsyncStatus, DepositRecord, EscrowRecord, RedeemRecord } from "../types";
import { paymentApi } from "../services/api";

type PaymentState = {
  deposits: DepositRecord[];
  escrows: EscrowRecord[];
  redeems: RedeemRecord[];
  balance: string | null;
  status: AsyncStatus;
  error: string | null;
  fetchAll: () => Promise<void>;
  registerDeposit: (amount: string) => Promise<string>;
  requestEscrow: (amount: string) => Promise<string>;
  requestRedeem: (amount: string) => Promise<string>;
};

const getErrorMessage = (error: unknown, fallback: string) => (error instanceof Error ? error.message : fallback);

export const usePaymentStore = create<PaymentState>((set, get) => ({
  deposits: [],
  escrows: [],
  redeems: [],
  balance: null,
  status: "idle",
  error: null,
  fetchAll: async () => {
    set({ status: "loading", error: null });
    try {
      const [depositsResponse, escrowsResponse, redeemsResponse, balanceResponse] = await Promise.all([
        paymentApi.listDeposits(),
        paymentApi.listEscrows(),
        paymentApi.listRedeems(),
        paymentApi.getBalance(),
      ]);

      set({
        deposits: depositsResponse.deposits,
        escrows: escrowsResponse.escrows,
        redeems: redeemsResponse.redeems,
        balance: balanceResponse.balance,
        status: "idle",
      });
    } catch (error) {
      set({ status: "error", error: getErrorMessage(error, "Unable to load payment data") });
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
      const message = getErrorMessage(error, "Unable to register deposit");
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
      const message = getErrorMessage(error, "Unable to request escrow");
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
      const message = getErrorMessage(error, "Unable to request redeem");
      set({ status: "error", error: message });
      throw new Error(message);
    }
  },
}));

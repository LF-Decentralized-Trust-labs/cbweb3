// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { paymentApi } from "../services/api";
import type {
  ApproveEscrowResponse,
  ApproveRedeemResponse,
  AsyncStatus,
  DepositRecord,
  EscrowRecord,
  FiatExchangeResponse,
  RedeemRecord,
} from "../types";

type PaymentStore = {
  deposits: DepositRecord[];
  escrows: EscrowRecord[];
  redeems: RedeemRecord[];
  status: AsyncStatus;
  error: string | null;
  fetchAll: () => Promise<void>;
  fetchDeposits: (requesterId?: string) => Promise<void>;
  fetchEscrows: (requesterId?: string) => Promise<void>;
  fetchRedeems: (requesterId?: string) => Promise<void>;
  approveDeposit: (depositId: string) => Promise<void>;
  rejectDeposit: (depositId: string, reason: string) => Promise<void>;
  requestFiatExchange: (depositId: string) => Promise<FiatExchangeResponse>;
  approveEscrow: (escrowId: string) => Promise<ApproveEscrowResponse>;
  rejectEscrow: (escrowId: string, reason: string) => Promise<void>;
  approveRedeem: (redeemId: string) => Promise<ApproveRedeemResponse>;
  rejectRedeem: (redeemId: string, reason: string) => Promise<void>;
};

const getErrorMessage = (error: unknown, fallback: string) => (error instanceof Error ? error.message : fallback);

export const usePaymentStore = create<PaymentStore>((set) => ({
  deposits: [],
  escrows: [],
  redeems: [],
  status: "idle",
  error: null,
  fetchAll: async () => {
    set({ status: "loading", error: null });
    try {
      const [depositsResponse, escrowsResponse, redeemsResponse] = await Promise.all([
        paymentApi.listDeposits(),
        paymentApi.listEscrows(),
        paymentApi.listRedeems(),
      ]);
      set({
        deposits: depositsResponse.deposits,
        escrows: escrowsResponse.escrows,
        redeems: redeemsResponse.redeems,
        status: "idle",
      });
    } catch (error) {
      set({ status: "error", error: getErrorMessage(error, "Unable to load payment records") });
    }
  },
  fetchDeposits: async (requesterId) => {
    set({ status: "loading", error: null });
    try {
      const response = await paymentApi.listDeposits(requesterId);
      set({ deposits: response.deposits, status: "idle" });
    } catch (error) {
      set({ status: "error", error: getErrorMessage(error, "Unable to load deposits") });
    }
  },
  fetchEscrows: async (requesterId) => {
    set({ status: "loading", error: null });
    try {
      const response = await paymentApi.listEscrows(requesterId);
      set({ escrows: response.escrows, status: "idle" });
    } catch (error) {
      set({ status: "error", error: getErrorMessage(error, "Unable to load escrows") });
    }
  },
  fetchRedeems: async (requesterId) => {
    set({ status: "loading", error: null });
    try {
      const response = await paymentApi.listRedeems(requesterId);
      set({ redeems: response.redeems, status: "idle" });
    } catch (error) {
      set({ status: "error", error: getErrorMessage(error, "Unable to load redeems") });
    }
  },
  approveDeposit: async (depositId) => {
    set({ status: "loading", error: null });
    try {
      await paymentApi.approveDeposit({ deposit_id: depositId });
      const response = await paymentApi.listDeposits();
      set({ deposits: response.deposits, status: "idle" });
    } catch (error) {
      const message = getErrorMessage(error, "Unable to approve deposit");
      set({ status: "error", error: message });
      throw new Error(message);
    }
  },
  rejectDeposit: async (depositId, reason) => {
    set({ status: "loading", error: null });
    try {
      await paymentApi.rejectDeposit({ deposit_id: depositId, reason });
      const response = await paymentApi.listDeposits();
      set({ deposits: response.deposits, status: "idle" });
    } catch (error) {
      const message = getErrorMessage(error, "Unable to reject deposit");
      set({ status: "error", error: message });
      throw new Error(message);
    }
  },
  requestFiatExchange: async (depositId) => {
    set({ status: "loading", error: null });
    try {
      const response = await paymentApi.requestFiatExchange({ deposit_id: depositId });
      const depositsResponse = await paymentApi.listDeposits();
      set({ deposits: depositsResponse.deposits, status: "idle" });
      return response;
    } catch (error) {
      const message = getErrorMessage(error, "Unable to execute fiat exchange");
      set({ status: "error", error: message });
      throw new Error(message);
    }
  },
  approveEscrow: async (escrowId) => {
    set({ status: "loading", error: null });
    try {
      const response = await paymentApi.approveEscrow({ escrow_id: escrowId });
      const escrowsResponse = await paymentApi.listEscrows();
      set({ escrows: escrowsResponse.escrows, status: "idle" });
      return response;
    } catch (error) {
      const message = getErrorMessage(error, "Unable to approve escrow");
      set({ status: "error", error: message });
      throw new Error(message);
    }
  },
  rejectEscrow: async (escrowId, reason) => {
    set({ status: "loading", error: null });
    try {
      await paymentApi.rejectEscrow({ escrow_id: escrowId, reason });
      const escrowsResponse = await paymentApi.listEscrows();
      set({ escrows: escrowsResponse.escrows, status: "idle" });
    } catch (error) {
      const message = getErrorMessage(error, "Unable to reject escrow");
      set({ status: "error", error: message });
      throw new Error(message);
    }
  },
  approveRedeem: async (redeemId) => {
    set({ status: "loading", error: null });
    try {
      const response = await paymentApi.approveRedeem({ redeem_id: redeemId });
      const redeemsResponse = await paymentApi.listRedeems();
      set({ redeems: redeemsResponse.redeems, status: "idle" });
      return response;
    } catch (error) {
      const message = getErrorMessage(error, "Unable to approve redeem");
      set({ status: "error", error: message });
      throw new Error(message);
    }
  },
  rejectRedeem: async (redeemId, reason) => {
    set({ status: "loading", error: null });
    try {
      await paymentApi.rejectRedeem({ redeem_id: redeemId, reason });
      const redeemsResponse = await paymentApi.listRedeems();
      set({ redeems: redeemsResponse.redeems, status: "idle" });
    } catch (error) {
      const message = getErrorMessage(error, "Unable to reject redeem");
      set({ status: "error", error: message });
      throw new Error(message);
    }
  },
}));

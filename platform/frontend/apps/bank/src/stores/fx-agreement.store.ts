import { create } from "zustand";
import { fxAgreementApi } from "../services/api/fx-agreement.api";
import type {
  FXAgreement,
  ProposeFXAgreementRequest,
  ProposeFXAgreementResponse,
} from "../types";

type FXAgreementState = {
  agreements: FXAgreement[];
  currentAgreement: FXAgreement | null;
  status: "idle" | "loading" | "error";
  error: string | null;
  fetchAll: (params?: { counterparty?: string; state?: string }) => Promise<void>;
  getAgreement: (tradeId: string) => Promise<FXAgreement>;
  propose: (payload: ProposeFXAgreementRequest) => Promise<ProposeFXAgreementResponse>;
  accept: (tradeId: string) => Promise<void>;
  reject: (tradeId: string) => Promise<void>;
  cancel: (tradeId: string) => Promise<void>;
  settle: (tradeId: string) => Promise<void>;
  reset: () => void;
};

export const useFxAgreementStore = create<FXAgreementState>((set) => ({
  agreements: [],
  currentAgreement: null,
  status: "idle",
  error: null,
  fetchAll: async (params) => {
    set({ status: "loading", error: null });
    try {
      const agreements = await fxAgreementApi.list(params);
      set({ agreements, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load agreements" });
    }
  },
  getAgreement: async (tradeId) => {
    set({ status: "loading", error: null });
    try {
      const agreement = await fxAgreementApi.get(tradeId);
      set({ currentAgreement: agreement, status: "idle" });
      return agreement;
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load agreement" });
      throw error;
    }
  },
  propose: async (payload) => {
    set({ status: "loading", error: null });
    try {
      const response = await fxAgreementApi.propose(payload);
      const agreements = await fxAgreementApi.list();
      set({ agreements, status: "idle" });
      return response;
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to propose agreement" });
      throw error;
    }
  },
  accept: async (tradeId) => {
    set({ status: "loading", error: null });
    try {
      await fxAgreementApi.accept(tradeId);
      const agreement = await fxAgreementApi.get(tradeId);
      set({ currentAgreement: agreement, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to accept agreement" });
      throw error;
    }
  },
  reject: async (tradeId) => {
    set({ status: "loading", error: null });
    try {
      await fxAgreementApi.reject(tradeId);
      const agreement = await fxAgreementApi.get(tradeId);
      set({ currentAgreement: agreement, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to reject agreement" });
      throw error;
    }
  },
  cancel: async (tradeId) => {
    set({ status: "loading", error: null });
    try {
      await fxAgreementApi.cancel(tradeId);
      const agreement = await fxAgreementApi.get(tradeId);
      set({ currentAgreement: agreement, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to cancel agreement" });
      throw error;
    }
  },
  settle: async (tradeId) => {
    set({ status: "loading", error: null });
    try {
      await fxAgreementApi.settle(tradeId);
      const agreement = await fxAgreementApi.get(tradeId);
      set({ currentAgreement: agreement, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to settle agreement" });
      throw error;
    }
  },
  reset: () => {
    set({ agreements: [], currentAgreement: null, status: "idle", error: null });
  },
}));

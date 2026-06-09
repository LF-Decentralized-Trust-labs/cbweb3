import { create } from "zustand";
import { htlcApi } from "../services/api";
import { saveSecret } from "../services/htlc-secrets";
import type {
  HTLCLock,
  LockHTLCRequest,
  LockHTLCResponse,
  LockWithHashHTLCRequest,
  SearchHTLCParams,
  SearchHTLCResponse,
} from "../types";

type HTLCState = {
  locks: HTLCLock[];
  total: number;
  status: "idle" | "loading" | "error";
  error: string | null;
  fetchAll: () => Promise<void>;
  search: (params: SearchHTLCParams) => Promise<SearchHTLCResponse>;
  getStatus: (contractId: string) => Promise<HTLCLock>;
  lock: (payload: LockHTLCRequest) => Promise<LockHTLCResponse>;
  lockWithHash: (payload: LockWithHashHTLCRequest) => Promise<LockHTLCResponse>;
  settle: (contractId: string, secret: string) => Promise<void>;
  refund: (contractId: string) => Promise<void>;
};

export const useHtlcStore = create<HTLCState>((set) => ({
  locks: [],
  total: 0,
  status: "idle",
  error: null,
  fetchAll: async () => {
    set({ status: "loading", error: null });
    try {
      const response = await htlcApi.search({});
      set({ locks: response.locks, total: response.total, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load HTLC data" });
    }
  },
  search: async (params) => {
    return htlcApi.search(params);
  },
  getStatus: async (contractId) => {
    return htlcApi.getStatus(contractId);
  },
  lock: async (payload) => {
    set({ status: "loading", error: null });
    try {
      const response = await htlcApi.lock(payload);
      if (response.secret && response.contract_id) {
        saveSecret(response.contract_id, response.secret);
      }
      const searchResponse = await htlcApi.search({});
      set({ locks: searchResponse.locks, total: searchResponse.total, status: "idle" });
      return response;
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to lock funds" });
      throw error;
    }
  },
  lockWithHash: async (payload) => {
    set({ status: "loading", error: null });
    try {
      const response = await htlcApi.lockWithHash(payload);
      if (response.secret && response.contract_id) {
        saveSecret(response.contract_id, response.secret);
      }
      const searchResponse = await htlcApi.search({});
      set({ locks: searchResponse.locks, total: searchResponse.total, status: "idle" });
      return response;
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to lock funds with hash" });
      throw error;
    }
  },
  settle: async (contractId, secret) => {
    set({ status: "loading", error: null });
    try {
      await htlcApi.settle({ contract_id: contractId, secret });
      try {
        const response = await htlcApi.search({});
        set({ locks: response.locks, total: response.total, status: "idle" });
      } catch {
        set({ status: "idle" });
      }
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to settle lock" });
      throw error;
    }
  },
  refund: async (contractId) => {
    set({ status: "loading", error: null });
    try {
      await htlcApi.refund(contractId);
      try {
        const response = await htlcApi.search({});
        set({ locks: response.locks, total: response.total, status: "idle" });
      } catch {
        set({ status: "idle" });
      }
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to refund lock" });
      throw error;
    }
  },
}));

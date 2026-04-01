import { create } from "zustand";
import { htlcApi } from "../services/api";
import type {
  HTLCLock,
  LockHTLCRequest,
  LockHTLCResponse,
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
      const searchResponse = await htlcApi.search({});
      set({ locks: searchResponse.locks, total: searchResponse.total, status: "idle" });
      return response;
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to lock funds" });
      throw error;
    }
  },
  settle: async (contractId, secret) => {
    set({ status: "loading", error: null });
    try {
      await htlcApi.settle({ contract_id: contractId, secret });
      const response = await htlcApi.search({});
      set({ locks: response.locks, total: response.total, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to settle lock" });
      throw error;
    }
  },
  refund: async (contractId) => {
    set({ status: "loading", error: null });
    try {
      await htlcApi.refund(contractId);
      const response = await htlcApi.search({});
      set({ locks: response.locks, total: response.total, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to refund lock" });
      throw error;
    }
  },
}));

import CryptoJS from "crypto-js";
import { create } from "zustand";
import { htlcApi } from "../services/api";
import type { CreateAgreementRequest, FXAgreement, HTLCLock } from "../types";

type HTLCState = {
  agreements: FXAgreement[];
  locks: HTLCLock[];
  status: "idle" | "loading" | "error";
  error: string | null;
  fetch: () => Promise<void>;
  createAgreement: (payload: CreateAgreementRequest) => Promise<string | null>;
  lockFunds: (agreementId: string, secret: string) => Promise<string>;
  settle: (lockId: string, secret: string) => Promise<void>;
};

const hashSecret = (secret: string) => CryptoJS.SHA256(secret).toString();

export const useHtlcStore = create<HTLCState>((set) => ({
  agreements: [],
  locks: [],
  status: "idle",
  error: null,
  fetch: async () => {
    set({ status: "loading", error: null });
    try {
      const [agreements, locks] = await Promise.all([htlcApi.getAgreements(), htlcApi.getLocks()]);
      set({ agreements, locks, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load HTLC data" });
    }
  },
  createAgreement: async (payload) => {
    set({ status: "loading", error: null });
    try {
      const created = await htlcApi.createAgreement(payload);
      const agreements = await htlcApi.getAgreements();
      set({ agreements, status: "idle" });
      return created.id;
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to create agreement" });
      return null;
    }
  },
  lockFunds: async (agreementId, secret) => {
    set({ status: "loading", error: null });
    try {
      const hashLock = hashSecret(secret);
      await htlcApi.lockFunds(agreementId, hashLock);
      const locks = await htlcApi.getLocks();
      set({ locks, status: "idle" });
      return hashLock;
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to lock funds" });
      return "";
    }
  },
  settle: async (lockId, secret) => {
    set({ status: "loading", error: null });
    try {
      await htlcApi.settle(lockId, secret);
      const locks = await htlcApi.getLocks();
      set({ locks, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to settle lock" });
    }
  },
}));

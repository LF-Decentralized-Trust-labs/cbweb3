import { create } from "zustand";
import { kycApi } from "../services/api";
import type { AsyncStatus, IssueCredentialPayload, IssuedCredential } from "../types";

type KycState = {
  credentials: IssuedCredential[];
  status: AsyncStatus;
  error: string | null;
  fetch: () => Promise<void>;
  issueCredential: (payload: IssueCredentialPayload) => Promise<void>;
};

export const useKycStore = create<KycState>((set) => ({
  credentials: [],
  status: "idle",
  error: null,
  fetch: async () => {
    set({ status: "loading", error: null });
    try {
      const credentials = await kycApi.list();
      set({ credentials, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load credentials" });
    }
  },
  issueCredential: async (payload) => {
    set({ status: "loading", error: null });
    try {
      await kycApi.issue(payload);
      const credentials = await kycApi.list();
      set({ credentials, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to issue credential" });
    }
  },
}));

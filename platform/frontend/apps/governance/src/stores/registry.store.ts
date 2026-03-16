import { create } from "zustand";
import { registryApi } from "../services/api";
import type { AsyncStatus, IssueCredentialPayload, Participant } from "../types";

type RegistryState = {
  participants: Participant[];
  status: AsyncStatus;
  error: string | null;
  fetch: () => Promise<void>;
  issueCredential: (payload: IssueCredentialPayload) => Promise<void>;
};

export const useRegistryStore = create<RegistryState>((set, get) => ({
  participants: [],
  status: "idle",
  error: null,
  fetch: async () => {
    set({ status: "loading", error: null });
    try {
      const participants = await registryApi.list();
      set({ participants, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to load registry" });
    }
  },
  issueCredential: async (payload) => {
    set({ status: "loading", error: null });
    try {
      await registryApi.issueCredential(payload);
      await get().fetch();
      set({ status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to issue credential" });
    }
  },
}));

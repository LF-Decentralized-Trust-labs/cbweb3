import { create } from "zustand";
import { governanceApi } from "../services/api";
import type {
  CredentialRevocationRequest,
  InstitutionalOnboardingRequest,
  Participant,
  SanctionsCheckResponse,
} from "../types";

type ParticipantsState = {
  participants: Participant[];
  lastSanctionsCheck: SanctionsCheckResponse | null;
  status: "idle" | "loading" | "error";
  error: string | null;
  fetch: () => Promise<void>;
  onboard: (payload: InstitutionalOnboardingRequest) => Promise<void>;
  revokeCredential: (payload: CredentialRevocationRequest) => Promise<void>;
  checkSanctions: (address: string) => Promise<void>;
};

export const useParticipantsStore = create<ParticipantsState>((set, get) => ({
  participants: [],
  lastSanctionsCheck: null,
  status: "idle",
  error: null,
  fetch: async () => {
    set({ status: "loading", error: null });
    try {
      const participants = await governanceApi.listParticipants();
      set({ participants, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to fetch participants" });
    }
  },
  onboard: async (payload) => {
    set({ status: "loading", error: null });
    try {
      await governanceApi.onboardInstitution(payload);
      await get().fetch();
      set({ status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to onboard participant" });
    }
  },
  revokeCredential: async (payload) => {
    set({ status: "loading", error: null });
    try {
      await governanceApi.revokeCredential(payload);
      await get().fetch();
      set({ status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to revoke credential" });
    }
  },
  checkSanctions: async (address) => {
    set({ status: "loading", error: null });
    try {
      const result = await governanceApi.checkSanctions(address);
      set({ lastSanctionsCheck: result, status: "idle" });
    } catch (error) {
      set({ status: "error", error: error instanceof Error ? error.message : "Unable to check sanctions" });
    }
  },
}));

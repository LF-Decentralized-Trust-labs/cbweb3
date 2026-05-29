import axios from "axios";
import { create } from "zustand";
import { oversightApi } from "../../services/api/oversight.api";
import type {
  DisclosureRequest,
  OpenDisclosureRequest,
  SignDisclosureRequest,
} from "../../types/oversight.types";

type OversightStore = {
  disclosures: DisclosureRequest[];
  currentDisclosure: DisclosureRequest | null;
  status: "idle" | "loading" | "error";
  error: string | null;
  openDisclosure: (payload: OpenDisclosureRequest) => Promise<void>;
  signDisclosure: (payload: SignDisclosureRequest) => Promise<void>;
  fetchDisclosureStatus: (requestId: string) => Promise<void>;
};

function extractApiError(error: unknown, fallback: string): string {
  if (axios.isAxiosError(error)) {
    const apiError = error.response?.data as { error?: string; message?: string } | undefined;
    if (apiError?.error) {
      return apiError.error;
    }
    if (apiError?.message) {
      return apiError.message;
    }
  }
  return error instanceof Error ? error.message : fallback;
}

export const useOversightStore = create<OversightStore>((set) => ({
  disclosures: [],
  currentDisclosure: null,
  status: "idle",
  error: null,
  openDisclosure: async (payload) => {
    set({ status: "loading", error: null });
    try {
      const disclosure = await oversightApi.openDisclosure(payload);
      set((state) => ({
        disclosures: [disclosure, ...state.disclosures],
        currentDisclosure: disclosure,
        status: "idle",
      }));
    } catch (error) {
      set({ status: "error", error: extractApiError(error, "Unable to open disclosure request") });
    }
  },
  signDisclosure: async (payload) => {
    set({ status: "loading", error: null });
    try {
      await oversightApi.signDisclosure(payload);
      set({ status: "idle" });
    } catch (error) {
      set({ status: "error", error: extractApiError(error, "Unable to sign disclosure request") });
    }
  },
  fetchDisclosureStatus: async (requestId) => {
    set({ status: "loading", error: null });
    try {
      const currentDisclosure = await oversightApi.getDisclosureStatus(requestId);
      set({ currentDisclosure, status: "idle" });
    } catch (error) {
      set({ status: "error", error: extractApiError(error, "Unable to fetch disclosure status") });
    }
  },
}));

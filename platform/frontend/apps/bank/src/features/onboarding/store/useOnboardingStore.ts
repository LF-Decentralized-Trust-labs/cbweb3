import { create } from "zustand";
import { onboardingApi } from "../../../services/api";
import type {
  AsyncStatus,
  CompleteOnboardingResponse,
  InitiateOnboardingPayload,
  OnboardingRequestStatus,
  OnboardingStatusResponse,
} from "../../../types";

type Step = 1 | 2 | 3 | 4;

type OnboardingState = {
  currentStep: Step;
  requestStatus: OnboardingRequestStatus | null;
  status: AsyncStatus;
  completionStatus: AsyncStatus;
  error: string | null;
  requestId: string | null;
  userId: string | null;
  walletAddress: string | null;
  popNonce: string | null;
  clientSecret: string | null;
  txHash: string | null;
  accessToken: string | null;
  pkiLoginError: string | null;
  startedAt: number | null;
  setCurrentStep: (step: Step) => void;
  start: () => void;
  initiate: (payload: InitiateOnboardingPayload) => Promise<boolean>;
  fetchStatus: () => Promise<OnboardingStatusResponse | null>;
  complete: () => Promise<CompleteOnboardingResponse | null>;
  reset: () => void;
};

const initialState = {
  currentStep: 1 as Step,
  requestStatus: null,
  status: "idle" as AsyncStatus,
  completionStatus: "idle" as AsyncStatus,
  error: null,
  requestId: null,
  userId: null,
  walletAddress: null,
  popNonce: null,
  clientSecret: null,
  txHash: null,
  accessToken: null,
  pkiLoginError: null,
  startedAt: null,
};

export const useOnboardingStore = create<OnboardingState>((set, get) => ({
  ...initialState,
  setCurrentStep: (step) => set({ currentStep: step }),
  start: () => set({ currentStep: 2, error: null }),
  initiate: async (payload) => {
    set({ status: "loading", error: null });

    try {
      const response = await onboardingApi.initiate(payload);
      set({
        requestId: response.request_id,
        userId: response.user_id,
        walletAddress: response.wallet_address,
        requestStatus: response.status,
        popNonce: null,
        clientSecret: null,
        txHash: null,
        accessToken: null,
        pkiLoginError: null,
        startedAt: Date.now(),
        currentStep: 3,
        status: "idle",
      });
      return true;
    } catch (error) {
      set({
        status: "error",
        error: error instanceof Error ? error.message : "Unable to initiate onboarding",
      });
      return false;
    }
  },
  fetchStatus: async () => {
    const requestId = get().requestId;
    if (!requestId) return null;

    try {
      const response = await onboardingApi.getStatus(requestId);
      set({
        requestStatus: response.status,
        popNonce: response.pop_nonce ?? null,
        currentStep: response.status === "KYC_APPROVED" ? 4 : get().currentStep,
        error: null,
      });
      return response;
    } catch (error) {
      set({ error: error instanceof Error ? error.message : "Unable to fetch onboarding status" });
      return null;
    }
  },
  complete: async () => {
    const { requestId, userId, completionStatus } = get();
    if (!requestId || !userId || completionStatus === "loading") return null;

    set({ completionStatus: "loading", error: null });

    try {
      const response = await onboardingApi.complete({
        request_id: requestId,
        user_id: userId,
      });

      set({
        requestStatus: response.status,
        walletAddress: response.wallet_address,
        clientSecret: response.client_secret,
        txHash: response.tx_hash,
        accessToken: response.access_token ?? null,
        pkiLoginError: response.pki_login_error ?? null,
        completionStatus: "idle",
        currentStep: 4,
      });

      return response;
    } catch (error) {
      set({
        completionStatus: "error",
        error: error instanceof Error ? error.message : "Unable to complete onboarding",
      });
      return null;
    }
  },
  reset: () => {
    set({ ...initialState });
  },
}));
import type {
  CompleteOnboardingPayload,
  CompleteOnboardingResponse,
  InitiateOnboardingPayload,
  InitiateOnboardingResponse,
  OnboardingStatusResponse,
} from "../../types";
import { httpClient } from "./http-client";

export const onboardingApi = {
  initiate: async (payload: InitiateOnboardingPayload): Promise<InitiateOnboardingResponse> => {
    const response = await httpClient.post<InitiateOnboardingResponse>("/onboarding/initiate", payload);
    return response.data;
  },
  getStatus: async (requestId: string): Promise<OnboardingStatusResponse> => {
    const response = await httpClient.get<OnboardingStatusResponse>(`/onboarding/status/${requestId}`);
    return response.data;
  },
  complete: async (payload: CompleteOnboardingPayload): Promise<CompleteOnboardingResponse> => {
    const response = await httpClient.post<CompleteOnboardingResponse>("/onboarding/complete", payload);
    return response.data;
  },
};
// SPDX-License-Identifier: Apache-2.0

import axios from "axios";
import { create } from "zustand";
import { circuitBreakerV2Api } from "../../services/api/circuit-breaker-v2.api";
import type {
  CircuitBreakerV2Status,
  PauseRequest,
  ProposeResumeRequest,
  SignResumeRequest,
} from "../../types/circuit-breaker-v2.types";

type CircuitBreakerV2Store = {
  cbStatus: CircuitBreakerV2Status | null;
  resumeRequestId: string | null;
  isStale: boolean;
  status: "idle" | "loading" | "error";
  error: string | null;
  fetchStatus: (pair: string) => Promise<void>;
  pause: (payload: PauseRequest) => Promise<void>;
  proposeResume: (payload: ProposeResumeRequest) => Promise<void>;
  signResume: (payload: SignResumeRequest) => Promise<void>;
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

export const useCircuitBreakerV2Store = create<CircuitBreakerV2Store>((set) => ({
  cbStatus: null,
  resumeRequestId: null,
  isStale: false,
  status: "idle",
  error: null,
  fetchStatus: async (pair) => {
    set({ status: "loading", error: null });
    try {
      const cbStatus = await circuitBreakerV2Api.getStatus(pair);
      set({ cbStatus, isStale: false, status: "idle" });
    } catch (error) {
      set((state) => ({
        status: "error",
        error: extractApiError(error, "Unable to fetch circuit breaker status"),
        isStale: state.cbStatus !== null,
      }));
    }
  },
  pause: async (payload) => {
    set({ status: "loading", error: null });
    try {
      const cbStatus = await circuitBreakerV2Api.pause(payload);
      set({ cbStatus, isStale: false, status: "idle" });
    } catch (error) {
      set({ status: "error", error: extractApiError(error, "Unable to pause circuit breaker") });
    }
  },
  proposeResume: async (payload) => {
    set({ status: "loading", error: null });
    try {
      const response = await circuitBreakerV2Api.proposeResume(payload);
      set((state) => ({
        cbStatus: state.cbStatus
          ? {
              ...state.cbStatus,
              state: response.state,
              resume_request_id: response.request_id,
            }
          : {
              pair: payload.pair,
              state: response.state,
              pause_initiator: null,
              pause_reason: null,
              resume_request_id: response.request_id,
            },
        resumeRequestId: response.request_id,
        isStale: false,
        status: "idle",
      }));
    } catch (error) {
      set({ status: "error", error: extractApiError(error, "Unable to propose resume") });
    }
  },
  signResume: async (payload) => {
    set({ status: "loading", error: null });
    try {
      const cbStatus = await circuitBreakerV2Api.signResume(payload);
      set({ cbStatus, isStale: false, status: "idle" });
    } catch (error) {
      set({ status: "error", error: extractApiError(error, "Unable to sign resume") });
    }
  },
}));

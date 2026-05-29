import type {
  CircuitBreakerV2Status,
  PauseRequest,
  ProposeResumeRequest,
  ProposeResumeResponse,
  SignResumeRequest,
} from "../../types/circuit-breaker-v2.types";
import { httpClientV2 } from "./http-client";

export const circuitBreakerV2Api = {
  getStatus: async (pair: string): Promise<CircuitBreakerV2Status> => {
    const response = await httpClientV2.get<CircuitBreakerV2Status>(
      "/governance/circuit-breaker/status",
      { params: { pair } },
    );
    return response.data;
  },
  pause: async (payload: PauseRequest): Promise<CircuitBreakerV2Status> => {
    const response = await httpClientV2.post<CircuitBreakerV2Status>(
      "/governance/circuit-breaker/pause",
      payload,
    );
    return response.data;
  },
  proposeResume: async (payload: ProposeResumeRequest): Promise<ProposeResumeResponse> => {
    const response = await httpClientV2.post<ProposeResumeResponse>(
      "/governance/circuit-breaker/resume-request",
      payload,
    );
    return response.data;
  },
  signResume: async (payload: SignResumeRequest): Promise<CircuitBreakerV2Status> => {
    const response = await httpClientV2.post<CircuitBreakerV2Status>(
      "/governance/circuit-breaker/resume-sign",
      payload,
    );
    return response.data;
  },
  normalizeOperationalStatus: (
    status: CircuitBreakerV2Status,
    stale: boolean,
  ): { severity: "info" | "warning" | "critical"; guidance: string } => {
    if (status.state === "HALTED") {
      return {
        severity: "critical",
        guidance: "Swaps are halted. Coordinate resume-request and multi-signature approval.",
      };
    }
    if (stale) {
      return {
        severity: "warning",
        guidance: "Showing last known circuit-breaker state. Retry status refresh.",
      };
    }
    if (status.state === "RESUME_PENDING") {
      return {
        severity: "warning",
        guidance: "Resume request is pending signatures.",
      };
    }
    return {
      severity: "info",
      guidance: "Circuit breaker is live.",
    };
  },
};

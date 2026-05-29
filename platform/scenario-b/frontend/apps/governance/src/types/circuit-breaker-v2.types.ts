export const CB_STATE = {
  LIVE: "LIVE",
  HALTED: "HALTED",
  RESUME_PENDING: "RESUME_PENDING",
} as const;

export type CbState = (typeof CB_STATE)[keyof typeof CB_STATE];

export interface CircuitBreakerV2Status {
  pair: string;
  state: CbState;
  pause_initiator: string | null;
  pause_reason: string | null;
  resume_request_id: string | null;
}

export interface PauseRequest {
  pair: string;
  bank_id: string;
  reason_code: string;
  signature: string;
}

export interface ProposeResumeRequest {
  pair: string;
  bank_id: string;
  signature: string;
}

export interface ProposeResumeResponse {
  request_id: string;
  state: CbState;
}

export interface SignResumeRequest {
  pair: string;
  request_id: string;
  bank_id: string;
  signature: string;
}

export interface CircuitBreakerOperationalStatus {
  state: CbState;
  stale: boolean;
  severity: "info" | "warning" | "critical";
  guidance: string;
}

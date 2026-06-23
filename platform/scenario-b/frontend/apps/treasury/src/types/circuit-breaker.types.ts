// SPDX-License-Identifier: Apache-2.0

export const CB_STATE = {
  LIVE: "LIVE",
  HALTED: "HALTED",
  RESUME_PENDING: "RESUME_PENDING",
} as const;

export type CbState = (typeof CB_STATE)[keyof typeof CB_STATE];

export interface CircuitBreakerStatus {
  pair: string;
  state: CbState;
  pause_initiator: string | null;
  pause_reason: string | null;
  resume_request_id: string | null;
}

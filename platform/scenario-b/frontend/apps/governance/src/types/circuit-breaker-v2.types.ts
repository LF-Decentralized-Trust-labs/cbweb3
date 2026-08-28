// SPDX-License-Identifier: Apache-2.0

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
  // 2-of-N progress of an in-flight resume proposal (surfaced on-chain to every CB).
  resume_signatures?: number;
  resume_quorum?: number;
  // On-chain reference of the pair's most recent breaker action, by any institution.
  // Undefined means this environment records no on-chain reference for the pair — which is
  // deliberately distinct from an empty string, so "no chain wired" never reads as a value
  // that failed to display. The API omits the key rather than sending "".
  tx_hash?: string;
}

// The institutional signature is generated server-side (CB PKI key), so it is not part
// of these request payloads anymore.
export interface PauseRequest {
  pair: string;
  bank_id: string;
  reason_code: string;
}

export interface ProposeResumeRequest {
  pair: string;
  bank_id: string;
}

// The resume proposal returns its own shape, so it needs the reference declared separately
// from CircuitBreakerV2Status — otherwise the proposal's hash is unrepresentable.
export interface ProposeResumeResponse {
  request_id: string;
  state: CbState;
  // The transaction that created the proposal. Distinct from request_id, which names the
  // proposal a co-signer must submit to resume-sign; neither substitutes for the other.
  tx_hash?: string;
}

export interface SignResumeRequest {
  pair: string;
  request_id: string;
  bank_id: string;
}

export interface CircuitBreakerOperationalStatus {
  state: CbState;
  stale: boolean;
  severity: "info" | "warning" | "critical";
  guidance: string;
}

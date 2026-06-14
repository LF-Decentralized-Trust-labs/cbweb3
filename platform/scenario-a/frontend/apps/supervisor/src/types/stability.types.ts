export type HTLCState =
  | "HTLC_STATE_PENDING"
  | "HTLC_STATE_LOCKED"
  | "HTLC_STATE_SETTLING"
  | "HTLC_STATE_SETTLED"
  | "HTLC_STATE_REFUNDING"
  | "HTLC_STATE_REFUNDED"
  | "HTLC_STATE_INVALID";

export interface HTLCSummary {
  id: string;
  state: HTLCState;
  hashLock: string;
  zetoLockRef: string;
  expiresAt: string;
}

export interface CircuitBreakerRequest {
  action: "PAUSE" | "RESUME";
  reason: string;
}

export interface StabilityAlert {
  id: string;
  severity: "LOW" | "MEDIUM" | "HIGH" | "CRITICAL";
  message: string;
  createdAt: string;
}

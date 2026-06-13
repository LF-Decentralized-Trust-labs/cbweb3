export type HTLCState = "LOCKED" | "REVEALED" | "REFUNDED" | "EXPIRED";

export interface HTLCSummary {
  id: string;
  state: HTLCState;
  amount: number;
  currency: string;
  counterparty: string;
  expiresAt: string;
  createdAt: string;
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

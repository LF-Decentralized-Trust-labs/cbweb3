export type HTLCState = "LOCKED" | "REVEALED" | "REFUNDED" | "EXPIRED";

export interface HTLCSummary {
  id: string;
  state: HTLCState;
  sender: string;
  receiver: string;
  expiresAt: string;
  counterpartyLocked: boolean;
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

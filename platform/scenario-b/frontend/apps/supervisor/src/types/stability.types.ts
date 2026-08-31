// SPDX-License-Identifier: Apache-2.0

export interface PoolStatus {
  pair: string;
  reserveA: number;
  reserveB: number;
  ratioA: number;
  ratioB: number;
  isImbalanced: boolean;
  updatedAt: string;
}

export interface CircuitBreakerRequest {
  action: "PAUSE" | "RESUME";
  reason: string;
}

export interface AMMConfigRequest {
  feeBps: number;
  slippageBps: number;
  reason: string;
}

export interface StabilityAlert {
  id: string;
  pair: string;
  severity: "LOW" | "MEDIUM" | "HIGH" | "CRITICAL";
  message: string;
  createdAt: string;
}

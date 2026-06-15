import type { PoolStatus, StabilityAlert } from "../../types";
import { apiFetch } from "./apiClient";

interface CircuitBreakerStatus {
  state: string;
  last_toggled_at?: string;
  reason?: string;
}

async function fetchCircuitBreakerStatus(): Promise<CircuitBreakerStatus | null> {
  try {
    return await apiFetch<CircuitBreakerStatus>("/api/v2/governance/circuit-breaker/status");
  } catch {
    return null;
  }
}

export const stabilityApi = {
  getPoolStatuses: async (): Promise<PoolStatus[]> => {
    const cb = await fetchCircuitBreakerStatus();
    const paused = cb?.state === "PAUSED";
    return [
      {
        pair: "W-BRL-ARS",
        reserveA: 0,
        reserveB: 0,
        ratioA: 1,
        ratioB: 1,
        isImbalanced: paused,
        updatedAt: cb?.last_toggled_at ?? new Date().toISOString(),
      },
    ];
  },

  getAlerts: async (): Promise<StabilityAlert[]> => {
    const cb = await fetchCircuitBreakerStatus();
    if (cb?.state === "PAUSED") {
      return [
        {
          id: "circuit-breaker-paused",
          pair: "ALL",
          severity: "CRITICAL",
          message: `Circuit breaker is PAUSED${cb.reason ? `: ${cb.reason}` : ""}`,
          createdAt: cb.last_toggled_at ?? new Date().toISOString(),
        },
      ];
    }
    return [];
  },
};

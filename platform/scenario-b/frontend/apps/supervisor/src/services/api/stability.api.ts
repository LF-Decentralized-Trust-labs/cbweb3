import type { PoolStatus, StabilityAlert } from "../../types";
import { apiFetch } from "./apiClient";

interface CircuitBreakerStatus {
  state: string;
  last_toggled_at?: string;
  reason?: string;
}

export const stabilityApi = {
  getPoolStatuses: async (): Promise<PoolStatus[]> => {
    try {
      // Real AMM pool data is not yet exposed via a supervisor-accessible endpoint.
      // GET /api/v1/governance/parameters returns AMM config (feeBps/slippageBps)
      // but not live pool reserves. Return empty until a pool-status endpoint is added.
      await apiFetch<unknown>("/api/v1/governance/parameters");
      return [];
    } catch {
      return [];
    }
  },

  getAlerts: async (): Promise<StabilityAlert[]> => {
    try {
      const status = await apiFetch<CircuitBreakerStatus>("/api/v1/governance/circuit-breaker/status");
      if (status.state === "PAUSED") {
        return [
          {
            id: "circuit-breaker-paused",
            pair: "ALL",
            severity: "CRITICAL",
            message: `Circuit breaker is PAUSED${status.reason ? `: ${status.reason}` : ""}`,
            createdAt: status.last_toggled_at ?? new Date().toISOString(),
          },
        ];
      }
      return [];
    } catch {
      return [];
    }
  },
};

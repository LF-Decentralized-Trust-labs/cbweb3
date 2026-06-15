import type { NetworkOverview } from "../../types";
import { apiFetch } from "./apiClient";

interface ParticipantsResponse {
  participants: Array<{ status: string }>;
}

interface CircuitBreakerStatus {
  state: string;
}

export const networkApi = {
  getOverview: async (): Promise<NetworkOverview> => {
    const [participantsResult, cbResult] = await Promise.allSettled([
      apiFetch<ParticipantsResponse>("/api/v1/compliance/participants/summary"),
      apiFetch<CircuitBreakerStatus>("/api/v1/governance/circuit-breaker/status"),
    ]);

    const participants =
      participantsResult.status === "fulfilled"
        ? participantsResult.value.participants
        : [];

    const cbPaused =
      cbResult.status === "fulfilled" && cbResult.value.state === "PAUSED";

    return {
      totalSupply: 0,
      activeInstitutions: participants.filter((p) => p.status === "ACTIVE").length,
      activeAgreements: 0,
      healthyPools: cbPaused ? 0 : 1,
      imbalancedPools: cbPaused ? 1 : 0,
      lastUpdatedAt: new Date().toISOString(),
    };
  },
};

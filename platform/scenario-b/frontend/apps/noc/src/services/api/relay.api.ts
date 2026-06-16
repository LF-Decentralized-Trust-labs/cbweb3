import type { RelayStatus } from "../../types";
import { httpClient } from "./http-client";

// Backend relay metric shape from GET /api/v1/relays
type BackendRelayMetric = {
  id: string;
  route: string;
  latency_p50_ms: number | null;
  latency_p95_ms: number | null;
  proof_success_rate_pct: number;
  status: "HEALTHY" | "DEGRADED" | "DOWN";
  updated_at: string;
};

export const relayApi = {
  list: async (): Promise<RelayStatus[]> => {
    const res = await httpClient.get<{ data: BackendRelayMetric[] }>("/relays");
    const metrics: BackendRelayMetric[] = res.data.data ?? [];
    return metrics.map((m) => ({
      id: m.id,
      route: m.route,
      latencyP50Ms: m.latency_p50_ms ?? 0,
      latencyP95Ms: m.latency_p95_ms ?? 0,
      proofSuccessRatePct: m.proof_success_rate_pct,
      status: m.status,
      updatedAt: m.updated_at,
    }));
  },
};

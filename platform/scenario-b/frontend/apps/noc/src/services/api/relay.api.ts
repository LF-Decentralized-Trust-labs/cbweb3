// SPDX-License-Identifier: Apache-2.0

import type { RelayStatus } from "../../types";
import { httpClient } from "./http-client";

type DataEnvelope<T> = { data: T };

type RelayMetricResponse = {
  id: string;
  route: string;
  latency_p50_ms: number | null;
  latency_p95_ms: number | null;
  proof_success_rate_pct: number;
  status: "HEALTHY" | "DEGRADED" | "DOWN";
  updated_at: string;
};

function toRelayStatus(r: RelayMetricResponse): RelayStatus {
  return {
    id: r.id,
    route: r.route,
    latencyP50Ms: r.latency_p50_ms,
    latencyP95Ms: r.latency_p95_ms,
    proofSuccessRatePct: r.proof_success_rate_pct,
    status: r.status,
    updatedAt: r.updated_at,
  };
}

export const relayApi = {
  list: async (window?: "24h" | "7d"): Promise<RelayStatus[]> => {
    const params = window ? `?window=${window}` : "";
    const res = await httpClient.get<DataEnvelope<RelayMetricResponse[]>>(`/relays${params}`);
    return res.data.data.map(toRelayStatus);
  },
};

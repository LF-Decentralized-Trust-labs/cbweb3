// SPDX-License-Identifier: Apache-2.0

import type { PoolFetchFailure, PoolStatus } from "../../types";
import { httpClient } from "./http-client";

// Backend pool shape from GET /api/v1/pools
type BackendPoolStatus = {
  pair: string;
  reserveA: string;
  reserveB: string;
  ratioA: number;
  ratioB: number;
  breached7030: boolean;
  severity: "INFO" | "CRITICAL";
  updatedAt: string;
};

// Pairs the backend could not read from the api-gateway. Reported so an unreachable or
// misconfigured gateway does not look like a deployment with no pools.
type BackendPoolFailure = {
  pair: string;
  reason: string;
};

export type PoolListResult = {
  pools: PoolStatus[];
  failures: PoolFetchFailure[];
};

export const poolApi = {
  list: async (): Promise<PoolListResult> => {
    const res = await httpClient.get<{ data: BackendPoolStatus[]; failures?: BackendPoolFailure[] }>("/pools");
    const pools: BackendPoolStatus[] = res.data.data ?? [];
    return {
      pools: pools.map((p) => ({
        pair: p.pair,
        reserveA: p.reserveA,
        reserveB: p.reserveB,
        ratioA: p.ratioA,
        ratioB: p.ratioB,
        breached7030: p.breached7030,
        severity: p.severity,
        updatedAt: p.updatedAt,
      })),
      failures: (res.data.failures ?? []).map((f) => ({ pair: f.pair, reason: f.reason })),
    };
  },
};

import type { PoolStatus } from "../../types";
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

export const poolApi = {
  list: async (): Promise<PoolStatus[]> => {
    const res = await httpClient.get<{ data: BackendPoolStatus[] }>("/pools");
    const pools: BackendPoolStatus[] = res.data.data ?? [];
    return pools.map((p) => ({
      pair: p.pair,
      reserveA: p.reserveA,
      reserveB: p.reserveB,
      ratioA: p.ratioA,
      ratioB: p.ratioB,
      breached7030: p.breached7030,
      severity: p.severity,
      updatedAt: p.updatedAt,
    }));
  },
};

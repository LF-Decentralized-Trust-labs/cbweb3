import type { PoolStatus, StabilityAlert } from "../../types";
import { apiFetch } from "./apiClient";

interface CircuitBreakerStatus {
  state: string;
  last_toggled_at?: string;
  reason?: string;
}

interface AMMPoolStatus {
  pool_pair: string;
  pool_status: "EMPTY" | "PENDING_COUNTERPART" | "ACTIVE";
  reserve_a: string;
  reserve_b: string;
  current_ratio: number;
  imbalance_flag: boolean;
  updated_at: string;
}

async function fetchCircuitBreakerStatus(): Promise<CircuitBreakerStatus | null> {
  try {
    return await apiFetch<CircuitBreakerStatus>("/api/v2/governance/circuit-breaker/status");
  } catch {
    return null;
  }
}

async function fetchPoolStatus(pair: string): Promise<AMMPoolStatus | null> {
  try {
    return await apiFetch<AMMPoolStatus>(`/api/v2/amm/pool/${pair}/status`);
  } catch {
    return null;
  }
}

const KNOWN_PAIRS = ["W-BRL-ARS"];
const WEI = BigInt("1000000000000000000");

function fromWei(raw: string): number {
  if (!raw || raw === "0") return 0;
  try {
    const n = BigInt(raw);
    const whole = n / WEI;
    const frac = ((n % WEI) * BigInt(1_000_000)) / WEI;
    return parseFloat(`${whole}.${String(frac).padStart(6, "0")}`);
  } catch {
    return 0;
  }
}

export const stabilityApi = {
  getPoolStatuses: async (): Promise<PoolStatus[]> => {
    const results = await Promise.allSettled(KNOWN_PAIRS.map((pair) => fetchPoolStatus(pair)));
    const pools: PoolStatus[] = [];
    for (const result of results) {
      if (result.status !== "fulfilled" || !result.value) continue;
      const p = result.value;
      if (p.pool_status === "EMPTY") continue;
      pools.push({
        pair: p.pool_pair,
        reserveA: fromWei(p.reserve_a),
        reserveB: fromWei(p.reserve_b),
        ratioA: parseFloat(p.current_ratio.toFixed(4)),
        ratioB: 1,
        isImbalanced: p.imbalance_flag,
        updatedAt: p.updated_at,
      });
    }
    return pools;
  },

  getAlerts: async (): Promise<StabilityAlert[]> => {
    const [cbResult, ...poolResults] = await Promise.allSettled([
      fetchCircuitBreakerStatus(),
      ...KNOWN_PAIRS.map((pair) => fetchPoolStatus(pair)),
    ]);

    const alerts: StabilityAlert[] = [];

    if (cbResult.status === "fulfilled" && cbResult.value?.state === "PAUSED") {
      const cb = cbResult.value;
      alerts.push({
        id: "circuit-breaker-paused",
        pair: "ALL",
        severity: "CRITICAL",
        message: `Circuit breaker is PAUSED${cb.reason ? `: ${cb.reason}` : ""}`,
        createdAt: cb.last_toggled_at ?? new Date().toISOString(),
      });
    }

    for (const result of poolResults) {
      if (result.status !== "fulfilled" || !result.value) continue;
      const p = result.value;
      if (p.pool_status !== "EMPTY" && p.imbalance_flag) {
        alerts.push({
          id: `imbalance-${p.pool_pair}`,
          pair: p.pool_pair,
          severity: "HIGH",
          message: `Pool ${p.pool_pair} is imbalanced (ratio: ${p.current_ratio.toFixed(4)})`,
          createdAt: p.updated_at,
        });
      }
    }

    return alerts;
  },
};

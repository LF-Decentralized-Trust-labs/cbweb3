import type { PoolStatus, StabilityAlert } from "../../types";

// Scenario A (HTLC-based) has no AMM pools or liquidity alerts.
// These return empty arrays as a safe no-op.
export const stabilityApi = {
  getPoolStatuses: (): Promise<PoolStatus[]> => Promise.resolve([]),
  getAlerts: (): Promise<StabilityAlert[]> => Promise.resolve([]),
};

// SPDX-License-Identifier: Apache-2.0

export type PoolSeverity = "INFO" | "WARNING" | "CRITICAL";

export type PoolStatus = {
  pair: string;
  reserveA: string;
  reserveB: string;
  ratioA: number;
  ratioB: number;
  breached7030: boolean;
  severity: PoolSeverity;
  updatedAt: string;
};

/** A configured pair the NOC backend could not read from the api-gateway. */
export type PoolFetchFailure = {
  pair: string;
  reason: string;
};

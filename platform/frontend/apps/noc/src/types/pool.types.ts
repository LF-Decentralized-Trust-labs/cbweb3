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

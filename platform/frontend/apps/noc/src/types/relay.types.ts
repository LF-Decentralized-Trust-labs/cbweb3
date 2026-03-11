export type RelayStatus = {
  id: string;
  route: string;
  latencyP50Ms: number;
  latencyP95Ms: number;
  proofSuccessRatePct: number;
  status: "HEALTHY" | "DEGRADED" | "DOWN";
  updatedAt: string;
};

// SPDX-License-Identifier: Apache-2.0

export type RelayStatus = {
  id: string;
  route: string;
  latencyP50Ms: number | null;
  latencyP95Ms: number | null;
  proofSuccessRatePct: number;
  status: "HEALTHY" | "DEGRADED" | "DOWN";
  updatedAt: string;
};

// SPDX-License-Identifier: Apache-2.0

export type NodeHealthStatus = "HEALTHY" | "DEGRADED" | "DOWN";

export type InfrastructureNode = {
  id: string;
  network: string;
  region: string;
  component: "BESU" | "PALADIN";
  uptimePct: number;
  syncLagBlocks: number;
  status: NodeHealthStatus;
  updatedAt: string;
};

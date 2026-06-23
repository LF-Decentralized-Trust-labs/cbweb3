// SPDX-License-Identifier: Apache-2.0

export type TopologyNode = {
  id: string;
  label: string;
  kind: "BESU" | "PALADIN" | "CACTI" | "HUB";
  spoke_id: string;
  spoke_name: string;
  redundant: boolean;
  status: "HEALTHY" | "DEGRADED" | "DOWN";
};

export type TopologyEdge = {
  id: string;
  from: string;
  to: string;
  healthy: boolean;
};

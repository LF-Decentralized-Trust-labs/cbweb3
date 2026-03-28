export type TopologyNode = {
  id: string;
  label: string;
  kind: "BESU" | "PALADIN" | "CACTI" | "HUB";
  redundant: boolean;
  status: "HEALTHY" | "DEGRADED" | "DOWN";
};

export type TopologyEdge = {
  id: string;
  from: string;
  to: string;
  healthy: boolean;
};

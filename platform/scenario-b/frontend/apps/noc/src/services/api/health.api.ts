import type { InfrastructureNode, TelemetryFrame, TopologyEdge, TopologyNode } from "../../types";
import { httpClient } from "./http-client";
import { mockDb } from "../mocks/mock-db";

// Backend component shape from GET /api/v1/spokes/:spokeId/components
type BackendComponent = {
  id: string;
  name: string;
  type: string;
  health_status: string;
  last_block_number?: number | null;
  last_checked_at?: string | null;
};

type BackendSpoke = {
  id: string;
  name: string;
  currency_code: string;
  jurisdiction: string;
};

function mapStatus(s: string): "HEALTHY" | "DEGRADED" | "DOWN" {
  if (s === "HEALTHY") return "HEALTHY";
  if (s === "DEGRADED") return "DEGRADED";
  return "DOWN";
}

export const healthApi = {
  getInfrastructure: async (): Promise<InfrastructureNode[]> => {
    // Fetch all spokes, then fetch components for each spoke.
    const spokesRes = await httpClient.get<{ data: BackendSpoke[] }>("/spokes");
    const spokes: BackendSpoke[] = spokesRes.data.data ?? [];

    const nodes: InfrastructureNode[] = [];

    await Promise.all(
      spokes.map(async (spoke) => {
        const compsRes = await httpClient.get<{ data: BackendComponent[] }>(
          `/spokes/${spoke.id}/components`,
        );
        const components: BackendComponent[] = compsRes.data.data ?? [];

        for (const comp of components) {
          // Only BESU and PALADIN map to InfrastructureNode (CACTI_RELAY is shown in relays)
          if (comp.type !== "BESU" && comp.type !== "PALADIN") continue;

          nodes.push({
            id: comp.id,
            network: spoke.name,
            region: spoke.currency_code,
            component: comp.type as "BESU" | "PALADIN",
            uptimePct: 0, // Not computed server-side per component — use 0 as placeholder
            syncLagBlocks: 0, // Not computed server-side per component — use 0 as placeholder
            status: mapStatus(comp.health_status),
            updatedAt: comp.last_checked_at ?? new Date().toISOString(),
          });
        }
      }),
    );

    return nodes;
  },

  // Telemetry is WebSocket-based; keep as mock until WS is implemented.
  getTelemetrySnapshot: (): Promise<TelemetryFrame[]> => mockDb.getTelemetrySnapshot(),

  getTopology: async (): Promise<{ nodes: TopologyNode[]; edges: TopologyEdge[] }> => {
    const res = await httpClient.get<{ data: { nodes: TopologyNode[]; edges: TopologyEdge[] } }>(
      "/topology",
    );
    return res.data.data;
  },
};

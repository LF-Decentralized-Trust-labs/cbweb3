import type { InfrastructureNode, TelemetryFrame, TopologyEdge, TopologyNode } from "../../types";
import { mockDb } from "../mocks/mock-db";

export const healthApi = {
  getInfrastructure: (): Promise<InfrastructureNode[]> => mockDb.getInfrastructure(),
  getTelemetrySnapshot: (): Promise<TelemetryFrame[]> => mockDb.getTelemetrySnapshot(),
  getTopology: (): Promise<{ nodes: TopologyNode[]; edges: TopologyEdge[] }> => mockDb.getTopology(),
};

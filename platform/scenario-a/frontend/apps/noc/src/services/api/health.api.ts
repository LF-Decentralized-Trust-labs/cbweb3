import type { TopologyEdge, TopologyNode } from "../../types";
import { httpClient } from "./http-client";

type DataEnvelope<T> = { data: T };

export const healthApi = {
  getTopology: async (): Promise<{ nodes: TopologyNode[]; edges: TopologyEdge[] }> => {
    const res = await httpClient.get<DataEnvelope<{ nodes: TopologyNode[]; edges: TopologyEdge[] }>>("/topology");
    return res.data.data;
  },
};

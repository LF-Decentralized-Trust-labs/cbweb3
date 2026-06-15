import type { SpokeHubDelta, TvlSnapshot } from "../../types";
import { httpClient } from "./http-client";

type TvlResponse = {
  snapshots: TvlSnapshot[];
};

export const reconciliationApi = {
  getTvl: async (): Promise<TvlSnapshot[]> => {
    const response = await httpClient.get<TvlResponse>("/reconciliation/tvl");
    return response.data.snapshots ?? [];
  },
  getDelta: async (): Promise<SpokeHubDelta> => {
    const response = await httpClient.get<SpokeHubDelta>("/reconciliation/delta");
    return response.data;
  },
};

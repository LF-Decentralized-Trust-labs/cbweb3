import type { SpokeHubDelta, TvlSnapshot } from "../../types";
import { mockDb } from "../mocks/mock-db";

export const reconciliationApi = {
  getTvl: (): Promise<TvlSnapshot[]> => mockDb.getTvlSnapshots(),
  getDelta: (): Promise<SpokeHubDelta> => mockDb.getSpokeHubDelta(),
};

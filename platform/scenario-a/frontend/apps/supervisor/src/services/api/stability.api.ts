import type { HTLCSummary, StabilityAlert } from "../../types";

export const stabilityApi = {
  getHTLCs: (): Promise<HTLCSummary[]> => Promise.resolve([]),
  getAlerts: (): Promise<StabilityAlert[]> => Promise.resolve([]),
};

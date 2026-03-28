import { mockDb } from "../mocks/mock-db";

export const stabilityApi = {
  getPoolStatuses: () => mockDb.getPoolStatuses(),
  getAlerts: () => mockDb.getStabilityAlerts(),
};

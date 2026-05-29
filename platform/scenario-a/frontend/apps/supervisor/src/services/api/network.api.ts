import { mockDb } from "../mocks/mock-db";

export const networkApi = {
  getOverview: () => mockDb.getNetworkOverview(),
};

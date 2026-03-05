import { mockDb } from "../mocks/mock-db";
import type { AMMConfigRequest, CircuitBreakerRequest } from "../../types";

export const stabilityApi = {
  getPoolStatuses: () => mockDb.getPoolStatuses(),
  setCircuitBreaker: (payload: CircuitBreakerRequest) => mockDb.setCircuitBreaker(payload),
  updateAmmConfig: (payload: AMMConfigRequest) => mockDb.updateAmmConfig(payload),
  getAlerts: () => mockDb.getStabilityAlerts(),
};

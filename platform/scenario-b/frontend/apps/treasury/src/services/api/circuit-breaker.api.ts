import type { CircuitBreakerStatus } from "../../types/circuit-breaker.types";
import { httpClientV2 } from "./http-client";

// Read-only view of the hub circuit breaker for a pool pair. Treasury shares the
// governance Keycloak client, so this ROLE_GOVERNANCE endpoint is reachable.
// Pausing/resuming stays in the governance portal — treasury only displays state.
export const circuitBreakerApi = {
  getStatus: async (pair: string): Promise<CircuitBreakerStatus> => {
    const response = await httpClientV2.get<CircuitBreakerStatus>("/governance/circuit-breaker/status", {
      params: { pair },
    });
    return response.data;
  },
};

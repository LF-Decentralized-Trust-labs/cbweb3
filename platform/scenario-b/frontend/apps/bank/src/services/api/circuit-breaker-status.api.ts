// SPDX-License-Identifier: Apache-2.0

import { httpClientV2 } from "./http-client";

type CircuitBreakerStatusResponse = {
  pair: string;
  state: "LIVE" | "HALTED" | "RESUME_PENDING";
  pause_initiator: string | null;
  pause_reason: string | null;
  resume_request_id: string | null;
};

export const circuitBreakerStatusApi = {
  getStatus: async (pair: string): Promise<CircuitBreakerStatusResponse> => {
    const response = await httpClientV2.get<CircuitBreakerStatusResponse>(
      "/governance/circuit-breaker/status",
      { params: { pair } },
    );
    return response.data;
  },
};

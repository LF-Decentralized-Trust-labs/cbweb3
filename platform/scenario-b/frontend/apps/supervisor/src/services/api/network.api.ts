// SPDX-License-Identifier: Apache-2.0

import type { NetworkOverview } from "../../types";
import { apiFetch } from "./apiClient";

interface ParticipantsResponse {
  participants: Array<{ status: string }>;
}

interface AMMPoolStatus {
  pool_status: "EMPTY" | "PENDING_COUNTERPART" | "ACTIVE";
  imbalance_flag: boolean;
}

const KNOWN_PAIRS = ["W-BRL-ARS"];

export const networkApi = {
  getOverview: async (): Promise<NetworkOverview> => {
    const [participantsResult, ...poolResults] = await Promise.allSettled([
      apiFetch<ParticipantsResponse>("/api/v1/compliance/participants/summary"),
      ...KNOWN_PAIRS.map((pair) =>
        apiFetch<AMMPoolStatus>(`/api/v2/amm/pool/${pair}/status`),
      ),
    ]);

    const participants =
      participantsResult.status === "fulfilled"
        ? participantsResult.value.participants
        : [];

    let healthyPools = 0;
    let imbalancedPools = 0;
    for (const result of poolResults) {
      if (result.status !== "fulfilled") continue;
      const p = result.value;
      if (p.pool_status === "EMPTY") continue;
      if (p.imbalance_flag) {
        imbalancedPools++;
      } else {
        healthyPools++;
      }
    }

    return {
      totalSupply: 0,
      activeInstitutions: participants.filter((p) => p.status === "ACTIVE").length,
      activeAgreements: 0,
      healthyPools,
      imbalancedPools,
      lastUpdatedAt: new Date().toISOString(),
    };
  },
};

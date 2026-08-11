// SPDX-License-Identifier: Apache-2.0

import type { NetworkOverview } from "../../types";
import { apiFetch } from "./apiClient";
import { listActivePairIds } from "./pairs.api";
import { getSovereignSupply } from "./token-supply.api";

interface ParticipantsResponse {
  participants: Array<{ status: string }>;
}

interface AMMPoolStatus {
  pool_status: "EMPTY" | "PENDING_COUNTERPART" | "ACTIVE";
  imbalance_flag: boolean;
}

export const networkApi = {
  getOverview: async (): Promise<NetworkOverview> => {
    const [participantsResult, pairsResult, supplyResult] = await Promise.allSettled([
      apiFetch<ParticipantsResponse>("/api/v1/compliance/participants/summary"),
      listActivePairIds(),
      getSovereignSupply(),
    ]);

    const participants =
      participantsResult.status === "fulfilled"
        ? participantsResult.value.participants
        : [];

    const pairs = pairsResult.status === "fulfilled" ? pairsResult.value : [];
    const poolResults = await Promise.allSettled(
      pairs.map((pair) =>
        apiFetch<AMMPoolStatus>(
          `/api/v2/amm/pool/${encodeURIComponent(pair)}/status`,
        ),
      ),
    );

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
      // null (not 0) when the read fails — the card renders as unavailable.
      sovereignSupply: supplyResult.status === "fulfilled" ? supplyResult.value : null,
      activeInstitutions: participants.filter((p) => p.status === "ACTIVE").length,
      healthyPools,
      imbalancedPools,
      lastUpdatedAt: new Date().toISOString(),
    };
  },
};

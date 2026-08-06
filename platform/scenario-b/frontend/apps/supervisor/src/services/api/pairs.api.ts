// SPDX-License-Identifier: Apache-2.0

import { apiFetch } from "./apiClient";

interface PairEntry {
  pair_id: string;
  status: string;
}

interface PairsResponse {
  pairs?: PairEntry[];
}

// listActivePairIds returns the pair identifiers registered on the hub PairRegistry
// (GET /api/v2/amm/pairs — public, served from the DB view enriched on-chain). Every
// pool read in this portal is driven off this list so a corridor opened at runtime is
// supervised immediately, with no frontend release.
//
// PROPOSED pairs are excluded: they have no AMM liquidity yet, so a pool status call
// would only cost an RPC round trip to return EMPTY.
//
// Returns [] when the endpoint is unreachable. Callers must treat an empty list as
// "pool coverage unavailable", never as "the network has no pools".
export async function listActivePairIds(): Promise<string[]> {
  const response = await apiFetch<PairsResponse>("/api/v2/amm/pairs");
  return (response.pairs ?? [])
    .filter((pair) => pair.status === "ACTIVE")
    .map((pair) => pair.pair_id);
}

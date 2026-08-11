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
//
// A single Dashboard refresh asks three times (network overview, pool table, alerts),
// each fanning out one pool-status call per pair. Concurrent and back-to-back callers
// therefore share one in-flight request for CACHE_TTL_MS; a corridor opened at runtime
// still lands on the next refresh cycle.
const CACHE_TTL_MS = 2_000;

let inFlight: Promise<string[]> | null = null;
let cachedAt = 0;

export function listActivePairIds(): Promise<string[]> {
  const now = Date.now();
  if (inFlight && now - cachedAt < CACHE_TTL_MS) return inFlight;

  cachedAt = now;
  inFlight = apiFetch<PairsResponse>("/api/v2/amm/pairs")
    .then((response) =>
      (response.pairs ?? [])
        .filter((pair) => pair.status === "ACTIVE")
        .map((pair) => pair.pair_id),
    )
    .catch((error) => {
      // Never cache a failure: the next caller must retry, not inherit an empty list
      // for the rest of the window and render "no pools" on a healthy network.
      inFlight = null;
      throw error;
    });
  return inFlight;
}

// SPDX-License-Identifier: Apache-2.0

import { create } from "zustand";
import { ammPairsApi } from "../services/api/amm-pairs.api";
import { circuitBreakerV2Api } from "../services/api/circuit-breaker-v2.api";
import type { AsyncStatus, CircuitBreakerMode } from "../types";
import type { CbState } from "../types/circuit-breaker-v2.types";

// The at-a-glance breaker indicator in the portal chrome makes a network-wide claim ("swaps
// are globally halted"), but the authoritative breaker status is held per currency pair. The
// network-wide condition is therefore derived here from the same on-chain V2 status the
// Circuit Breaker page reads, rather than fetched from a separate endpoint. That is what stops
// the chrome and the page from ever contradicting each other about whether swaps are halted.
//
// It deliberately does NOT use the V1 `governanceApi.getCircuitBreaker` call, which branches
// on VITE_USE_MOCKS and would let the indicator show synthetic state while the page shows
// live state.

// A network-wide condition of `null` is indeterminate: the set of pairs is unknown, empty, or
// only partially readable. Indeterminate is never rendered as a halt — the indicator must not
// assert a stop it cannot substantiate.
export type NetworkBreakerCondition = { state: CircuitBreakerMode } | null;

type CircuitBreakerStore = {
  circuitBreaker: NetworkBreakerCondition;
  status: AsyncStatus;
  error: string | null;
  fetchState: () => Promise<void>;
};

// deriveNetworkBreakerCondition reduces per-pair states to the single network-wide claim.
// A `null` entry means that pair's status could not be read.
//
// Rules:
//  - halted if ANY known pair is halted — a halt anywhere halts the network claim;
//  - operational only when the pair set is known AND every pair was read AND none is halted;
//  - otherwise indeterminate (`null`): no pairs, or partial information.
//
// A pair awaiting resume quorum (RESUME_PENDING) counts as halted: it is still paused
// on-chain, so swaps are still stopped, and the breaker page shows it as such.
export function deriveNetworkBreakerCondition(
  pairStates: Array<CbState | null>,
): CircuitBreakerMode | null {
  if (pairStates.length === 0) {
    return null;
  }
  if (pairStates.some((state) => state === "HALTED" || state === "RESUME_PENDING")) {
    return "HALTED";
  }
  // No halt found, but an unreadable pair could be hiding one. Report indeterminate rather
  // than claiming the network is operational on partial information.
  if (pairStates.some((state) => state === null)) {
    return null;
  }
  return "LIVE";
}

export const useCircuitBreakerStore = create<CircuitBreakerStore>((set) => ({
  circuitBreaker: null,
  status: "idle",
  error: null,
  fetchState: async () => {
    set({ status: "loading", error: null });
    try {
      const pairs = await ammPairsApi.getPairs();
      if (pairs.length === 0) {
        // No pairs to judge: indeterminate, not halted, and not an error — a stack with no
        // pairs yet is a normal state and the chrome must still render.
        set({ circuitBreaker: null, status: "idle" });
        return;
      }

      // Pair counts on this platform are small, so reading each pair's authoritative status
      // is acceptable and needs no new aggregate endpoint.
      const results = await Promise.all(
        pairs.map(async (pair) => {
          try {
            const status = await circuitBreakerV2Api.getStatus(pair.pair_id);
            return status.state as CbState;
          } catch {
            // Treated as not-known-halted. The indeterminacy is surfaced below rather than
            // swallowed, so a partially readable network never reads as operational.
            return null;
          }
        }),
      );

      const state = deriveNetworkBreakerCondition(results);
      const unreadable = results.filter((result) => result === null).length;
      set({
        circuitBreaker: state ? { state } : null,
        status: "idle",
        error:
          unreadable > 0
            ? `Circuit breaker state unavailable for ${unreadable} of ${results.length} pairs`
            : null,
      });
    } catch (error) {
      // The pair set itself is unknown: stay indeterminate and report why.
      set({
        circuitBreaker: null,
        status: "error",
        error: error instanceof Error ? error.message : "Unable to load circuit breaker",
      });
    }
  },
}));

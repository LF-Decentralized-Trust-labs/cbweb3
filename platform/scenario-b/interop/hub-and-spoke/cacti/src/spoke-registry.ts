// SPDX-License-Identifier: Apache-2.0

/**
 * Dynamic spoke registry for the generalized relay (TK-B5). Replaces the fixed
 * two-spoke config: holds N ≥ 0 spokes indexed by id, drives connectors/watchers
 * and cross-currency routing, and is hydrated from the persisted RelayStore at
 * boot and mutated at runtime via POST /api/v1/spokes.
 */

export interface Spoke {
  /** Unique spoke id (e.g. "spoke-br"). */
  spokeId: string;
  /** HTTP JSON-RPC URL of the spoke Besu node. */
  besuRpc: string;
  /** WebSocket JSON-RPC URL of the spoke Besu node. */
  besuWs: string;
  /** Internal URL of the spoke's api-gateway (cross-currency routing target). */
  gatewayUrl: string;
}

export class SpokeRegistry {
  private readonly spokes = new Map<string, Spoke>();

  /** Load an initial set (boot hydration from the store). */
  hydrate(list: Spoke[]): void {
    for (const s of list) this.spokes.set(s.spokeId, s);
  }

  /** Idempotent upsert by spokeId (re-registering the same id does not duplicate). */
  upsert(s: Spoke): void {
    this.spokes.set(s.spokeId, s);
  }

  get(spokeId: string): Spoke | undefined {
    return this.spokes.get(spokeId);
  }

  list(): Spoke[] {
    return [...this.spokes.values()];
  }

  get size(): number {
    return this.spokes.size;
  }
}

// SPDX-License-Identifier: Apache-2.0

/**
 * Runtime spoke registration endpoint (TK-B5): POST /api/v1/spokes.
 * Validates the payload, upserts the registry (idempotent), persists the store,
 * and hydrates the runtime (connector/watcher/route) without a restart.
 *
 * Structural request/response types keep this module free of an express import
 * so it is unit-testable with plain mocks; index.ts adapts express handlers.
 */

import { SpokeRegistry, Spoke } from "./spoke-registry";
import { RelayStore } from "./relay-store";
import { validateSpoke } from "./config";

export interface ApiRequest {
  body: unknown;
}
export interface ApiResponse {
  status(code: number): ApiResponse;
  json(payload: unknown): void;
}

export interface SpokesApiDeps {
  registry: SpokeRegistry;
  store: RelayStore;
  /** Hydrate the runtime for a newly registered spoke (connector/watcher/route). */
  onRegister(spoke: Spoke): void;
}

export function makeSpokesHandler(deps: SpokesApiDeps) {
  return async (req: ApiRequest, res: ApiResponse): Promise<void> => {
    let spoke: Spoke;
    try {
      spoke = validateSpoke(req.body);
    } catch (err) {
      res.status(400).json({ error: (err as Error).message });
      return;
    }
    deps.registry.upsert(spoke); // idempotent by spokeId
    await deps.store.save(deps.registry.list());
    deps.onRegister(spoke);
    console.log(
      JSON.stringify({
        ts: new Date().toISOString(),
        service: "cacti-relay",
        severity: "INFO",
        event: "spoke_registered",
        spokeId: spoke.spokeId,
      }),
    );
    res.status(200).json({ status: "registered", spokeId: spoke.spokeId });
  };
}

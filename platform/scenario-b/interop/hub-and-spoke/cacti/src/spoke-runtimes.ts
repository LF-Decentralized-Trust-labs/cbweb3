// SPDX-License-Identifier: Apache-2.0

/**
 * Pure wiring for per-spoke runtimes (TK-B5). Extracted from index.ts so that
 * "one connector/watcher per spoke" (SC-001) is testable without a real Besu:
 * the connector/watcher factories are injected. Async because the real Cacti
 * factories init the connector and register web services.
 */

import { Spoke } from "./spoke-registry";

export interface SpokeRuntime {
  spokeId: string;
  connector: unknown;
  watcher: unknown;
  stop(): void;
}

export interface RuntimeFactories {
  createConnector(spoke: Spoke): Promise<{ connector: unknown; stop(): void | Promise<void> }>;
  createWatcher(spoke: Spoke): Promise<{ watcher: unknown; stop(): void | Promise<void> }>;
}

/** Build exactly one runtime (connector + watcher) per spoke, in order. */
export async function createSpokeRuntimes(
  spokes: Spoke[],
  factories: RuntimeFactories,
): Promise<SpokeRuntime[]> {
  const runtimes: SpokeRuntime[] = [];
  for (const spoke of spokes) {
    const c = await factories.createConnector(spoke);
    const w = await factories.createWatcher(spoke);
    runtimes.push({
      spokeId: spoke.spokeId,
      connector: c.connector,
      watcher: w.watcher,
      stop: () => {
        void c.stop();
        void w.stop();
      },
    });
  }
  return runtimes;
}

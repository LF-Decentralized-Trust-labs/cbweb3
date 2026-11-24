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
  /**
   * Stops the connector and the watcher and waits for both. Rejects with an AggregateError
   * listing every stop that failed, so a connector left open is reported, never swallowed.
   */
  stop(): Promise<void>;
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
    let w: Awaited<ReturnType<RuntimeFactories["createWatcher"]>>;
    try {
      w = await factories.createWatcher(spoke);
    } catch (watchErr) {
      // The connector already holds a Besu WebSocket; with no runtime to own it, nobody else
      // would ever close it.
      await stopAll(spoke.spokeId, [c.stop]).catch((closeErr: unknown) => {
        throw new AggregateError([watchErr, closeErr], `${spoke.spokeId}: watcher creation failed and stopping its connector also failed`);
      });
      throw watchErr;
    }
    runtimes.push({
      spokeId: spoke.spokeId,
      connector: c.connector,
      watcher: w.watcher,
      stop: () => stopAll(spoke.spokeId, [c.stop, w.stop]),
    });
  }
  return runtimes;
}

async function stopAll(spokeId: string, stops: Array<() => void | Promise<void>>): Promise<void> {
  const results = await Promise.allSettled(stops.map(async (stop) => stop()));
  const errors = results.flatMap((r) => (r.status === "rejected" ? [r.reason] : []));
  if (errors.length > 0) {
    throw new AggregateError(errors, `${spokeId}: ${errors.map(String).join("; ")}`);
  }
}

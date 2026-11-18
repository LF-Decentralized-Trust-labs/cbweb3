// SPDX-License-Identifier: Apache-2.0

import {
  IPluginLedgerConnectorBesuOptions,
  PluginLedgerConnectorBesu,
} from "@hyperledger/cactus-plugin-ledger-connector-besu";
// Not re-exported from the package root; the package declares no "exports" map, so the deep
// path is the supported way to reach the class the constructor option is typed with.
import { PrometheusExporter } from "@hyperledger/cactus-plugin-ledger-connector-besu/dist/lib/main/typescript/prometheus-exporter/prometheus-exporter";

// The slice of web3 1.x's WebsocketProvider this class relies on.
interface WsProviderLike {
  disconnect?: (code?: number, reason?: string) => void;
}

/**
 * One metrics exporter for every connector in the process.
 *
 * The library constructor builds a new PrometheusExporter per connector unless one is passed,
 * and calls startMetricsCollection() on it. That registers prom-client's default metrics on a
 * fresh registry, which starts a perf_hooks event-loop-delay histogram and a GC
 * PerformanceObserver. Nothing ever stops them, so a relay that rebuilds connectors during a
 * long Besu outage would accumulate one of each per rebuild. Sharing a single exporter, whose
 * collection starts once, bounds that cost to one set for the life of the process.
 */
class SharedPrometheusExporter extends PrometheusExporter {
  private started = false;

  startMetricsCollection(): void {
    if (this.started) return;
    this.started = true;
    super.startMetricsCollection();
  }
}

let sharedExporter: SharedPrometheusExporter | undefined;

/** The process-wide exporter every ClosingBesuConnector uses unless given its own. */
export function sharedBesuPrometheusExporter(): PrometheusExporter {
  sharedExporter ??= new SharedPrometheusExporter({ pollingIntervalInMin: 1 });
  return sharedExporter;
}

/**
 * A Cacti Besu connector that gives back what it takes.
 *
 * Cacti connector-besu 2.0.0 opens a WebSocket to Besu in its constructor and its shutdown()
 * only writes a log line, so a discarded connector keeps holding one of Besu's WebSocket slots
 * for as long as the relay process lives. The relay rebuilds a spoke's connector every few
 * seconds while poll cycles fail, which turns that into a steady leak that can exhaust the
 * node's connection limit and lock every other client out.
 *
 * Besu cannot reclaim these slots on its own: it sets no idle timeout and no TCP keepalive on
 * WebSocket connections, so a socket the client never closes stays counted indefinitely. The
 * client closing what it opened is the only place the slot is given back.
 *
 * The provider is a private field of the library class. If an upgrade renames it, shutdown()
 * throws instead of returning quietly, because a quiet no-op is exactly the original leak.
 * closing-besu-connector.test.ts pins the library behaviour, so an upstream fix is noticed and
 * this class can be removed.
 *
 * The metrics the library starts per instance are handled by sharing one exporter; see
 * sharedBesuPrometheusExporter().
 */
export class ClosingBesuConnector extends PluginLedgerConnectorBesu {
  constructor(options: IPluginLedgerConnectorBesuOptions) {
    super({ ...options, prometheusExporter: options.prometheusExporter ?? sharedBesuPrometheusExporter() });
  }

  // Set once the socket has been closed, so a second shutdown() (a rebuild that closed this
  // connector, then the exit path) does not close it again.
  private closed = false;

  async shutdown(): Promise<void> {
    if (this.closed) return;
    await super.shutdown();
    const provider = (this as unknown as { web3Provider?: WsProviderLike }).web3Provider;
    if (typeof provider?.disconnect !== "function") {
      throw new Error(
        `${this.getInstanceId()}: cannot close the Besu WebSocket, the connector library no ` +
          "longer exposes web3Provider.disconnect; the socket stays open on the Besu node",
      );
    }
    provider.disconnect(1000, "connector shutdown");
    this.closed = true;
  }

  /**
   * Builds and initialises a connector, closing it again if initialisation fails. The socket
   * is opened by the constructor, so a connector whose onPluginInit() throws would otherwise
   * hold its slot with nobody left to close it.
   */
  static async open(options: IPluginLedgerConnectorBesuOptions): Promise<ClosingBesuConnector> {
    const connector = new ClosingBesuConnector(options);
    try {
      await connector.onPluginInit();
    } catch (initErr) {
      try {
        await connector.shutdown();
      } catch (closeErr) {
        throw new AggregateError(
          [initErr, closeErr],
          `${options.instanceId}: initialisation failed and closing the connector also failed, ` +
            "its WebSocket may stay open on the Besu node",
        );
      }
      throw initErr;
    }
    return connector;
  }
}

/**
 * Closes every connector and waits for all of them, so the caller can exit knowing each close
 * either finished or failed. Returns the failures; it never throws.
 */
export async function closeConnectors(
  connectors: Map<string, { shutdown(): Promise<void> }>,
): Promise<Array<{ spokeId: string; error: unknown }>> {
  const entries = [...connectors];
  const results = await Promise.allSettled(entries.map(([, connector]) => connector.shutdown()));
  const failures: Array<{ spokeId: string; error: unknown }> = [];
  results.forEach((result, i) => {
    if (result.status === "rejected") failures.push({ spokeId: entries[i][0], error: result.reason });
  });
  return failures;
}

// SPDX-License-Identifier: Apache-2.0

import { PluginLedgerConnectorBesu } from "@hyperledger/cactus-plugin-ledger-connector-besu";

// The slice of web3 1.x's WebsocketProvider this class relies on.
interface WsProviderLike {
  disconnect?: (code?: number, reason?: string) => void;
}

/**
 * A Cacti Besu connector whose shutdown() actually closes its WebSocket.
 *
 * Cacti connector-besu 2.0.0 opens a WebSocket to Besu in its constructor and its shutdown()
 * only writes a log line, so a discarded connector keeps holding one of Besu's WebSocket slots
 * for as long as the relay process lives. The relay rebuilds a spoke's connector every few
 * seconds while poll cycles fail, which turns that into a steady leak. In SUP-152 the central
 * bank Besu ran out of all its slots, the local Paladin node could not open its block listener,
 * and the spoke settled nothing for fifteen days.
 *
 * Besu cannot reclaim these slots on its own: it sets no idle timeout and no TCP keepalive on
 * WebSocket connections, so a socket the client never closes stays counted indefinitely. The
 * client closing what it opened is the only place the slot is given back.
 *
 * The provider is a private field of the library class. If an upgrade renames it, shutdown()
 * throws instead of returning quietly, because a quiet no-op is exactly the original leak.
 * closing-besu-connector.test.ts pins the library behaviour, so an upstream fix is noticed and
 * this class can be removed.
 */
export class ClosingBesuConnector extends PluginLedgerConnectorBesu {
  async shutdown(): Promise<void> {
    await super.shutdown();
    const provider = (this as unknown as { web3Provider?: WsProviderLike }).web3Provider;
    if (typeof provider?.disconnect !== "function") {
      throw new Error(
        `${this.getInstanceId()}: cannot close the Besu WebSocket, the connector library no ` +
          "longer exposes web3Provider.disconnect; the socket stays open on the Besu node",
      );
    }
    provider.disconnect(1000, "connector shutdown");
  }
}

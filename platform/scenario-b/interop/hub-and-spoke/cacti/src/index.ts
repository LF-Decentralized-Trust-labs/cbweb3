/**
 * Cacti Liquidity Relay — entry point (Scenario B).
 *
 * Starts the LiquidityCommitWatcher to observe CommitMatched events from
 * LiquidityCommitRegistry on the Hub and notify configured gateways.
 *
 * Exposes a minimal REST API on CACTI_API_PORT (default 4000):
 *   GET  /api/v1/health  — liveness probe
 *
 * The @hyperledger/cactus-plugin-ledger-connector-besu plugin is initialised
 * here and attached to Express via registerWebServices, making standard
 * Cacti BesuConnector endpoints available (getPastLogs, getBlock, etc.)
 * and enabling watchBlocksV1 event streaming via Socket.IO.
 */

import http from "http";
import express, { Request, Response } from "express";
import { randomUUID } from "crypto";
import { Server as SocketIoServer } from "socket.io";
import { PluginRegistry } from "@hyperledger/cactus-core";
import { PluginLedgerConnectorBesu } from "@hyperledger/cactus-plugin-ledger-connector-besu";
import { config } from "./config";
import { createLiquidityCommitWatcherFromEnv } from "./liquidity-commit-watcher";
import { createCrossCurrencySwapRelayFromEnv } from "./cross-currency-swap-relay";


// ---------------------------------------------------------------------------
// Hub RPC readiness
// ---------------------------------------------------------------------------

/**
 * Poll the hub Besu JSON-RPC until it answers eth_blockNumber, so the watcher is only
 * started once the chain is actually reachable. This avoids the failure mode where the
 * relay starts before (or during a restart of) the hub Besu and the watcher races a node
 * that is not yet serving requests. Bounded wait — falls through after maxWaitMs so the
 * watcher's own self-healing (request timeouts + provider reconnect) takes over.
 */
async function waitForHubRpc(rpcUrl: string, maxWaitMs = 60_000, intervalMs = 2_000): Promise<void> {
  if (!rpcUrl) return;
  const deadline = Date.now() + maxWaitMs;
  for (;;) {
    try {
      const resp = await fetch(rpcUrl, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ jsonrpc: "2.0", method: "eth_blockNumber", params: [], id: 1 }),
        signal: AbortSignal.timeout(5_000),
      });
      if (resp.ok) {
        const json = (await resp.json().catch(() => ({}))) as Record<string, unknown>;
        if (json["result"]) {
          console.log(`[cacti] hub Besu RPC ready at ${rpcUrl} (block ${String(json["result"])})`);
          return;
        }
      }
    } catch {
      // not ready yet — keep polling until the deadline.
    }
    if (Date.now() >= deadline) {
      console.warn(
        `[cacti] hub Besu RPC not ready after ${maxWaitMs}ms — starting watcher anyway (it self-heals)`,
      );
      return;
    }
    await new Promise(r => setTimeout(r, intervalMs));
  }
}

// ---------------------------------------------------------------------------
// Bootstrap
// ---------------------------------------------------------------------------

async function main(): Promise<void> {
  console.log("Cacti Liquidity Relay starting (Scenario B)…");
  console.log(`  Spoke-A RPC  : ${config.spokeA.besuRpc}`);
  console.log(`  Spoke-A WS   : ${config.spokeA.besuWs}`);
  console.log(`  Spoke-B RPC  : ${config.spokeB.besuRpc}`);
  console.log(`  Spoke-B WS   : ${config.spokeB.besuWs}`);
  console.log(`  API port     : ${config.apiPort}`);

  // ── Cacti PluginRegistry + Besu connectors ──────────────────────────────
  const pluginRegistry = new PluginRegistry();

  const connectorSpokeA = new PluginLedgerConnectorBesu({
    instanceId: `besu-connector-spoke-a-${randomUUID()}`,
    rpcApiHttpHost: config.spokeA.besuRpc,
    rpcApiWsHost: config.spokeA.besuWs,
    pluginRegistry,
    logLevel: "INFO",
  });

  const connectorSpokeB = new PluginLedgerConnectorBesu({
    instanceId: `besu-connector-spoke-b-${randomUUID()}`,
    rpcApiHttpHost: config.spokeB.besuRpc,
    rpcApiWsHost: config.spokeB.besuWs,
    pluginRegistry,
    logLevel: "INFO",
  });

  await connectorSpokeA.onPluginInit();
  console.log(`[cacti] PluginLedgerConnectorBesu spoke-a initialized (${connectorSpokeA.getInstanceId()})`);

  await connectorSpokeB.onPluginInit();
  console.log(`[cacti] PluginLedgerConnectorBesu spoke-b initialized (${connectorSpokeB.getInstanceId()})`);

  // ── Start LiquidityCommitWatcher (007-bridge-based-cb-liquidity) ────────
  // The Hub is now an independent network (chain 1337) — always use the
  // HUB_BESU_RPC ethers provider, never Spoke-A's connector.
  const abortController = new AbortController();
  const lcrWatcher = createLiquidityCommitWatcherFromEnv();
  if (lcrWatcher) {
    // Wait for the hub chain to be reachable before polling, so a relay that comes up
    // before (or during a restart of) the hub Besu does not race an unavailable node.
    await waitForHubRpc(process.env["HUB_BESU_RPC"] ?? "");
    lcrWatcher.start(abortController.signal);
    console.log("[cacti] LiquidityCommitWatcher started");
  } else {
    console.warn("[cacti] LiquidityCommitWatcher not configured — relay will be idle");
  }

  // ── Cross-currency swap relay (009-commercial-cross-currency-swap) ────────
  const crossCurrencyRelay = createCrossCurrencySwapRelayFromEnv();
  if (crossCurrencyRelay) {
    console.log("[cacti] CrossCurrencySwapRelay started — CB-B bridge-out relay active");
  } else {
    console.warn("[cacti] CrossCurrencySwapRelay not configured — cross-currency bridge-out relay disabled");
  }

  // ── Express REST API ─────────────────────────────────────────────────────
  const app = express();
  app.use(express.json());

  // Liveness / readiness
  app.get("/api/v1/health", (_req: Request, res: Response) => {
    res.json({
      status: "ok",
      uptime: process.uptime(),
      mode: "scenario-b-liquidity",
      watcher_active: lcrWatcher !== null,
      cross_currency_relay_active: crossCurrencyRelay !== null,
    });
  });

  // Cross-currency bridge-out relay (009):
  //   POST /api/v1/cross-currency/bridge-out
  //   Called by CB-A api-gateway after Hub AMM swap to trigger CB-B bridge-out.
  if (crossCurrencyRelay) {
    app.post("/api/v1/cross-currency/bridge-out", crossCurrencyRelay.handleBridgeOut);
    console.log("[cacti] registered POST /api/v1/cross-currency/bridge-out");
  }

  // ── HTTP server ─────────────────────────────────────────────────────────
  const httpServer = http.createServer(app);
  const ioServer = new SocketIoServer(httpServer, {
    cors: { origin: "*" },
    path: "/api/v1/plugins/socket.io/",
  });

  // Register Cacti connector web services (REST + watchBlocksV1 Socket.IO)
  const endpointsA = await connectorSpokeA.registerWebServices(app, ioServer);
  console.log(`[cacti] spoke-a registered ${endpointsA.length} web service endpoint(s)`);
  const endpointsB = await connectorSpokeB.registerWebServices(app, ioServer);
  console.log(`[cacti] spoke-b registered ${endpointsB.length} web service endpoint(s)`);

  httpServer.listen(config.apiPort, () => {
    console.log(`Cacti Liquidity Relay API listening on :${config.apiPort}`);
  });

  // ── Graceful shutdown ───────────────────────────────────────────────────
  const shutdown = (): void => {
    console.log("Shutting down…");
    abortController.abort();
    lcrWatcher?.stop();
    // crossCurrencyRelay is stateless — no explicit stop needed.
    connectorSpokeA.shutdown().catch(() => {});
    connectorSpokeB.shutdown().catch(() => {});
    ioServer.close();
    httpServer.close(() => process.exit(0));
  };
  process.on("SIGINT", shutdown);
  process.on("SIGTERM", shutdown);
}

main().catch((err: unknown) => {
  console.error("Fatal:", err);
  process.exit(1);
});

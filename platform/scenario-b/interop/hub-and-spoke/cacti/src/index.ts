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
  // Uses connectorSpokeA as the Hub connector if HUB_BESU_RPC is not set,
  // or falls back to a direct ethers JsonRpcProvider if set.
  const abortController = new AbortController();
  const lcrWatcher = createLiquidityCommitWatcherFromEnv(connectorSpokeA);
  if (lcrWatcher) {
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

// SPDX-License-Identifier: Apache-2.0

/**
 * Cacti Liquidity Relay — entry point (Scenario B, generalized for N spokes, TK-B5).
 *
 * Boots NEUTRAL (zero spokes), hydrates the dynamic SpokeRegistry from the
 * persisted RelayStore (+ optional SPOKES_JSON), and creates one Besu
 * connector/watcher per spoke. New spokes register at runtime via
 * POST /api/v1/spokes (no restart). The LiquidityCommitWatcher on the Hub and
 * the cross-currency bridge-out relay remain; the latter routes by spoke_out
 * and validates the AMM circuit breaker (isPaused) before forwarding.
 */

import http from "http";
import express, { Request, Response } from "express";
import { randomUUID } from "crypto";
import { Server as SocketIoServer } from "socket.io";
import { PluginRegistry } from "@hyperledger/cactus-core";
import { PluginLedgerConnectorBesu } from "@hyperledger/cactus-plugin-ledger-connector-besu";
import { config, loadSpokesFromEnv } from "./config";
import { SpokeRegistry, Spoke } from "./spoke-registry";
import { RelayStore } from "./relay-store";
import { createSpokeRuntimes, RuntimeFactories, SpokeRuntime } from "./spoke-runtimes";
import { makeSpokesHandler } from "./spokes-api";
import { createLiquidityCommitWatcherFromEnv } from "./liquidity-commit-watcher";
import { createCrossCurrencySwapRelay } from "./cross-currency-swap-relay";
import { makeEthersIsPausedReader, checkNotPaused } from "./circuit-breaker";

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
      // not ready yet
    }
    if (Date.now() >= deadline) {
      console.warn(`[cacti] hub Besu RPC not ready after ${maxWaitMs}ms — continuing (self-heals)`);
      return;
    }
    await new Promise((r) => setTimeout(r, intervalMs));
  }
}

async function main(): Promise<void> {
  console.log("Cacti Liquidity Relay starting (Scenario B, N-spokes)…");
  console.log(`  API port     : ${config.apiPort}`);

  const pluginRegistry = new PluginRegistry();
  const registry = new SpokeRegistry();
  const store = new RelayStore(process.env["RELAY_STORE_PATH"] ?? "/data/relay-spokes.json");

  // ── Express + Socket.IO (created early so runtime factories can register) ──
  const app = express();
  app.use(express.json());
  const httpServer = http.createServer(app);
  const ioServer = new SocketIoServer(httpServer, {
    cors: { origin: "*" },
    path: "/api/v1/plugins/socket.io/",
  });

  const runtimes = new Map<string, SpokeRuntime>();

  // Real per-spoke factories: create + init a Besu connector and register its
  // web services (watchBlocksV1). The "watcher" handle mirrors the connector.
  const factories: RuntimeFactories = {
    async createConnector(spoke: Spoke) {
      const connector = new PluginLedgerConnectorBesu({
        instanceId: `besu-connector-${spoke.spokeId}-${randomUUID()}`,
        rpcApiHttpHost: spoke.besuRpc,
        rpcApiWsHost: spoke.besuWs,
        pluginRegistry,
        logLevel: "INFO",
      });
      await connector.onPluginInit();
      await connector.registerWebServices(app, ioServer);
      console.log(`[cacti] connector for ${spoke.spokeId} initialized`);
      return { connector, stop: () => connector.shutdown().catch(() => {}) };
    },
    async createWatcher(spoke: Spoke) {
      // Per-spoke watcher handle (web services registered with the connector).
      return { watcher: { spokeId: spoke.spokeId }, stop: () => {} };
    },
  };

  async function hydrateSpoke(spoke: Spoke): Promise<void> {
    if (runtimes.has(spoke.spokeId)) return; // idempotent: already running
    const [rt] = await createSpokeRuntimes([spoke], factories);
    runtimes.set(spoke.spokeId, rt);
  }

  // ── Boot neutro: hydrate registry from store + SPOKES_JSON, build runtimes ─
  const persisted = await store.load();
  const seeded = loadSpokesFromEnv();
  registry.hydrate([...persisted, ...seeded]);
  for (const spoke of registry.list()) {
    await hydrateSpoke(spoke);
  }
  console.log(`[cacti] booted with ${registry.size} spoke(s)`);

  // ── LiquidityCommitWatcher (Hub) ──────────────────────────────────────────
  const abortController = new AbortController();
  const lcrWatcher = createLiquidityCommitWatcherFromEnv();
  if (lcrWatcher) {
    await waitForHubRpc(process.env["HUB_BESU_RPC"] ?? "");
    lcrWatcher.start(abortController.signal);
    console.log("[cacti] LiquidityCommitWatcher started");
  } else {
    console.warn("[cacti] LiquidityCommitWatcher not configured — relay idle on hub events");
  }

  // ── Cross-currency relay: routes by spoke_out + isPaused gate ─────────────
  const hubRpc = process.env["HUB_BESU_RPC"] ?? "";
  const isPausedReader = makeEthersIsPausedReader(hubRpc);
  const notPaused = (amm: string | undefined) => checkNotPaused(amm, isPausedReader);
  const crossCurrencyRelay = createCrossCurrencySwapRelay(registry, notPaused);
  if (crossCurrencyRelay) {
    app.post("/api/v1/cross-currency/bridge-out", crossCurrencyRelay.handleBridgeOut);
    console.log("[cacti] registered POST /api/v1/cross-currency/bridge-out");
  } else {
    console.warn("[cacti] CrossCurrencySwapRelay disabled (missing INTERNAL_RELAY_AUTH_SECRET)");
  }

  // ── Runtime spoke registration (TK-B5): POST /api/v1/spokes ───────────────
  const spokesHandler = makeSpokesHandler({
    registry,
    store,
    onRegister: (spoke) => {
      void hydrateSpoke(spoke); // create connector/watcher without restart
    },
  });
  app.post("/api/v1/spokes", (req: Request, res: Response) => {
    void spokesHandler({ body: req.body }, {
      status: (c: number) => { res.status(c); return res as never; },
      json: (p: unknown) => res.json(p),
    });
  });
  console.log("[cacti] registered POST /api/v1/spokes");

  // Liveness
  app.get("/api/v1/health", (_req: Request, res: Response) => {
    res.json({
      status: "ok",
      uptime: process.uptime(),
      mode: "scenario-b-liquidity",
      spokes: registry.size,
      watcher_active: lcrWatcher !== null,
      cross_currency_relay_active: crossCurrencyRelay !== null,
    });
  });

  httpServer.listen(config.apiPort, () => {
    console.log(`Cacti Liquidity Relay API listening on :${config.apiPort}`);
  });

  const shutdown = (): void => {
    console.log("Shutting down…");
    abortController.abort();
    lcrWatcher?.stop();
    for (const rt of runtimes.values()) rt.stop();
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

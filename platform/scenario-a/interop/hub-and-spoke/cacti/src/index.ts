// SPDX-License-Identifier: Apache-2.0

/**
 * Cacti HTLC Relay — entry point.
 *
 * Starts the HtlcRelay (event watcher + gRPC SettleHTLC caller) and exposes
 * a REST API on CACTI_API_PORT (default 4000) so the CactiRelay Go adapter
 * inside each payment-orchestrator can:
 *
 *   GET  /api/v1/relay/events/settle?since=<unix_ms>   — poll settle events
 *   GET  /api/v1/relay/events/lock?since=<unix_ms>     — poll lock events
 *   POST /api/v1/relay/proof                           — store a relay proof
 *   GET  /api/v1/relay/proof/:correlationId            — retrieve a proof
 *   GET  /api/v1/health                                — liveness probe
 *
 * The @hyperledger/cactus-plugin-ledger-connector-besu plugin is initialised
 * here and attached to Express via registerWebServices, making standard
 * Cacti BesuConnector endpoints available (getPastLogs, getBlock, etc.)
 * and enabling watchBlocksV1 event streaming via Socket.IO.
 */

import http from "http";
import express, { Request, Response } from "express";
import { randomUUID, timingSafeEqual } from "crypto";
import { Server as SocketIoServer } from "socket.io";
import { PluginRegistry } from "@hyperledger/cactus-core";
import { PluginLedgerConnectorBesu } from "@hyperledger/cactus-plugin-ledger-connector-besu";
import { config } from "./config";
import { HtlcRelay } from "./htlc-relay";
import { RelayStore } from "./relay-store";

// ---------------------------------------------------------------------------
// In-memory proof store (RelayProof / VerifyProof for InteroperabilityPort)
// ---------------------------------------------------------------------------

interface RelayProof {
  correlationId: string;
  sourceChain: string;
  contractId: string;
  eventName: string;
  hashLock: string;
  timeLock: number;
  zetoLockRef: string;
  proofPayload: string; // base64-encoded bytes
  relayTxId: string;
  verified: boolean;
  createdAt: number;
}

const proofStore = new Map<string, RelayProof>();
const MAX_PROOFS = 5_000;

function pruneProofs(): void {
  if (proofStore.size > MAX_PROOFS) {
    const oldest = [...proofStore.keys()].slice(0, proofStore.size - MAX_PROOFS);
    for (const k of oldest) proofStore.delete(k);
  }
}

// ---------------------------------------------------------------------------
// Bootstrap
// ---------------------------------------------------------------------------

async function main(): Promise<void> {
  console.log("Cacti HTLC relay starting…");
  console.log(`  Poll interval: ${config.pollIntervalMs} ms`);
  console.log(`  API port     : ${config.apiPort}`);
  console.log(`  Store path   : ${config.relayStorePath}`);
  console.log(`  Spokes       : ${config.spokes.length}`);

  // ── Cacti PluginRegistry + Besu connectors ──────────────────────────────
  const pluginRegistry = new PluginRegistry();
  const connectors = new Map<string, PluginLedgerConnectorBesu>();

  for (const spoke of config.spokes) {
    const connector = new PluginLedgerConnectorBesu({
      instanceId: `besu-connector-${spoke.id}-${randomUUID()}`,
      rpcApiHttpHost: spoke.besuRpc,
      rpcApiWsHost: spoke.besuWs,
      pluginRegistry,
      logLevel: "INFO",
    });
    await connector.onPluginInit();
    connectors.set(spoke.id, connector);
    console.log(`[cacti] registered spoke: ${spoke.id} rpc=${spoke.besuRpc} htlc=${spoke.htlcAddress}`);
  }

  // ── Start HTLC relay ────────────────────────────────────────────────────
  const abortController = new AbortController();
  const relayStore = new RelayStore(config.relayStorePath);
  await relayStore.init();

  const relay = new HtlcRelay(
    config.spokes,
    config.protoPath,
    config.pollIntervalMs,
    config.relayAuthSecret,
    relayStore,
    connectors,
  );
  relay.start(abortController.signal);

  // ── Express REST API ────────────────────────────────────────────────────
  const app = express();
  app.use(express.json());

  // Guard all relay endpoints with a constant-time X-Relay-Auth header check.
  // Risk: information disclosure — unauthenticated callers could observe in-flight
  // and settled cross-chain payment flows (sender, receiver, hashlock, preimage).
  app.use("/api/v1/relay", (req: Request, res: Response, next) => {
    const provided = String(req.headers["x-relay-auth"] ?? "");
    const secret = config.relayAuthSecret;
    if (
      provided.length === 0 ||
      provided.length !== secret.length ||
      !timingSafeEqual(Buffer.from(provided), Buffer.from(secret))
    ) {
      res.status(401).json({ error: "X-Relay-Auth invalid or missing" });
      return;
    }
    next();
  });

  // Liveness / readiness
  app.get("/api/v1/health", (_req: Request, res: Response) => {
    res.json({ status: "ok", uptime: process.uptime() });
  });

  /**
   * GET /api/v1/relay/events/settle?since=<unix_ms>
   * Returns settle events observed since the given timestamp (0 = all).
   * Used by CactiRelay.SubscribeSettleEvents polling loop in Go.
   */
  app.get("/api/v1/relay/events/settle", (req: Request, res: Response) => {
    const since = parseInt(String(req.query["since"] ?? "0"), 10);
    res.json(relay.getSettleEvents(since));
  });

  /**
   * GET /api/v1/relay/events/lock?since=<unix_ms>
   * Returns lock events observed since the given timestamp (0 = all).
   * Used by CactiRelay.SubscribeLockEvents polling loop in Go.
   */
  app.get("/api/v1/relay/events/lock", (req: Request, res: Response) => {
    const since = parseInt(String(req.query["since"] ?? "0"), 10);
    res.json(relay.getLockEvents(since));
  });

  /**
   * GET /api/v1/relay/events/fx-proposed?since=<unix_ms>
   * Returns FX agreement proposal events observed since the given timestamp.
   */
  app.get("/api/v1/relay/events/fx-proposed", (req: Request, res: Response) => {
    const since = parseInt(String(req.query["since"] ?? "0"), 10);
    res.json(relay.getFXProposalEvents(since));
  });

  /**
   * GET /api/v1/relay/events/fx-accepted?since=<unix_ms>
   * Returns FX agreement acceptance events observed since the given timestamp.
   */
  app.get("/api/v1/relay/events/fx-accepted", (req: Request, res: Response) => {
    const since = parseInt(String(req.query["since"] ?? "0"), 10);
    res.json(relay.getFXAcceptanceEvents(since));
  });

  /**
   * GET /api/v1/relay/events/fx-rejected?since=<unix_ms>
   * Returns FX agreement rejection events observed since the given timestamp.
   */
  app.get("/api/v1/relay/events/fx-rejected", (req: Request, res: Response) => {
    const since = parseInt(String(req.query["since"] ?? "0"), 10);
    res.json(relay.getFXRejectionEvents(since));
  });

  /**
   * GET /api/v1/relay/events/fx-cancelled?since=<unix_ms>
   * Returns FX agreement cancellation events observed since the given timestamp.
   */
  app.get("/api/v1/relay/events/fx-cancelled", (req: Request, res: Response) => {
    const since = parseInt(String(req.query["since"] ?? "0"), 10);
    res.json(relay.getFXCancellationEvents(since));
  });

  /**
   * GET /api/v1/relay/events/fx-settled?since=<unix_ms>
   * Returns FX agreement settlement events observed since the given timestamp.
   */
  app.get("/api/v1/relay/events/fx-settled", (req: Request, res: Response) => {
    const since = parseInt(String(req.query["since"] ?? "0"), 10);
    res.json(relay.getFXSettlementEvents(since));
  });

  /**
   * POST /api/v1/relay/proof
   * Store a cross-chain proof submitted by a payment-orchestrator.
   * Called by CactiRelay.RelayProof in Go.
   */
  app.post("/api/v1/relay/proof", (req: Request, res: Response) => {
    const body = req.body as Partial<RelayProof>;
    if (!body.contractId || !body.sourceChain) {
      res.status(400).json({ error: "contractId and sourceChain are required" });
      return;
    }
    const proof: RelayProof = {
      correlationId: body.correlationId ?? randomUUID(),
      sourceChain: body.sourceChain,
      contractId: body.contractId,
      eventName: body.eventName ?? "",
      hashLock: body.hashLock ?? "",
      timeLock: body.timeLock ?? 0,
      zetoLockRef: body.zetoLockRef ?? "",
      proofPayload: body.proofPayload ?? "",
      relayTxId: `cacti-relay-${randomUUID()}`,
      verified: false,
      createdAt: Date.now(),
    };
    proofStore.set(proof.correlationId, proof);
    pruneProofs();
    console.log(`relay proof stored correlationId=${proof.correlationId} contractId=${proof.contractId}`);
    res.status(201).json({ relay_tx_id: proof.relayTxId, correlation_id: proof.correlationId });
  });

  /**
   * GET /api/v1/relay/proof/:correlationId
   * Retrieve a previously stored proof and mark it as verified.
   * Called by CactiRelay.VerifyProof in Go.
   */
  app.get("/api/v1/relay/proof/:correlationId", (req: Request, res: Response) => {
    const proof = proofStore.get(req.params["correlationId"] ?? "");
    if (!proof) {
      res.status(404).json({ error: "proof not found" });
      return;
    }
    proof.verified = true;
    res.json({ ...proof, verified: true });
  });

  // ── HTTP server ─────────────────────────────────────────────────────────
  const httpServer = http.createServer(app);
  const ioServer = new SocketIoServer(httpServer, {
    cors: { origin: config.socketIoAllowedOrigins.length > 0 ? config.socketIoAllowedOrigins : false },
    path: "/api/v1/plugins/socket.io/",
  });

  // Register Cacti connector web services (REST + watchBlocksV1 Socket.IO)
  for (const [spokeId, connector] of connectors) {
    const endpoints = await connector.registerWebServices(app, ioServer);
    console.log(`[cacti] ${spokeId} registered ${endpoints.length} web service endpoint(s)`);
  }

  httpServer.listen(config.apiPort, () => {
    console.log(`Cacti HTLC relay API listening on :${config.apiPort}`);
  });

  // ── Graceful shutdown ───────────────────────────────────────────────────
  const shutdown = (): void => {
    console.log("Shutting down…");
    abortController.abort();
    for (const connector of connectors.values()) {
      connector.shutdown().catch(() => {});
    }
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

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
import { randomUUID } from "crypto";
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
  console.log(`  Spoke-A RPC  : ${config.spokeA.besuRpc}`);
  console.log(`  Spoke-A WS   : ${config.spokeA.besuWs}`);
  console.log(`  Spoke-A HTLC : ${config.spokeA.htlcAddress}`);
  console.log(`  Spoke-A API  : ${config.spokeA.internalApiUrl}`);
  console.log(`  Spoke-B RPC  : ${config.spokeB.besuRpc}`);
  console.log(`  Spoke-B WS   : ${config.spokeB.besuWs}`);
  console.log(`  Spoke-B HTLC : ${config.spokeB.htlcAddress}`);
  console.log(`  Spoke-B API  : ${config.spokeB.internalApiUrl}`);
  console.log(`  Poll interval: ${config.pollIntervalMs} ms`);
  console.log(`  API port     : ${config.apiPort}`);
  console.log(`  Store path   : ${config.relayStorePath}`);

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

  // ── Start HTLC relay ────────────────────────────────────────────────────
  const abortController = new AbortController();
  const relayStore = new RelayStore(config.relayStorePath);
  await relayStore.init();

  const connectors = new Map<string, PluginLedgerConnectorBesu>();
  connectors.set("spoke-a", connectorSpokeA);
  connectors.set("spoke-b", connectorSpokeB);

  const relay = new HtlcRelay(
    [config.spokeA, config.spokeB],
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
    cors: { origin: "*" },
    path: "/api/v1/plugins/socket.io/",
  });

  // Register Cacti connector web services (REST + watchBlocksV1 Socket.IO)
  const endpointsA = await connectorSpokeA.registerWebServices(app, ioServer);
  console.log(`[cacti] spoke-a registered ${endpointsA.length} web service endpoint(s)`);
  const endpointsB = await connectorSpokeB.registerWebServices(app, ioServer);
  console.log(`[cacti] spoke-b registered ${endpointsB.length} web service endpoint(s)`);

  httpServer.listen(config.apiPort, () => {
    console.log(`Cacti HTLC relay API listening on :${config.apiPort}`);
  });

  // ── Graceful shutdown ───────────────────────────────────────────────────
  const shutdown = (): void => {
    console.log("Shutting down…");
    abortController.abort();
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

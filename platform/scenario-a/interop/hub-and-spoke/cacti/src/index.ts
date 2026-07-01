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
import { PluginRegistry } from "@hyperledger/cactus-core";
import { PluginLedgerConnectorBesu } from "@hyperledger/cactus-plugin-ledger-connector-besu";
import { config } from "./config";
import { HtlcRelay, SpokeDep } from "./htlc-relay";
import { RelayStore } from "./relay-store";
import { SpokeRegistry, normalizeSpokeRegistration, SpokeRegistrationError } from "./spoke-registry";

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

  // ── Cacti PluginRegistry + connector factory ────────────────────────────
  // The factory (re)creates a Besu connector on demand: the relay calls it when a spoke is
  // registered at runtime and again to reconnect after a dropped WebSocket. Connectors are used
  // in-process (getPastLogs/getBlock) — no per-connector HTTP web services are exposed.
  const pluginRegistry = new PluginRegistry();
  const connectors = new Map<string, PluginLedgerConnectorBesu>();
  const connectorFactory = async (spoke: SpokeDep): Promise<PluginLedgerConnectorBesu> => {
    const connector = new PluginLedgerConnectorBesu({
      instanceId: `besu-connector-${spoke.id}-${randomUUID()}`,
      rpcApiHttpHost: spoke.besuRpc,
      rpcApiWsHost: spoke.besuWs,
      pluginRegistry,
      logLevel: "INFO",
    });
    await connector.onPluginInit();
    return connector;
  };

  // ── Relay store + dynamic spoke registry ─────────────────────────────────
  const abortController = new AbortController();
  const relayStore = new RelayStore(config.relayStorePath);
  await relayStore.init();

  // The registry is the single source of watched spokes. A founding central bank self-registers
  // its spoke via POST /api/v1/spokes; registrations persist across restarts.
  const spokeRegistry = new SpokeRegistry(config.spokeRegistryPath);
  await spokeRegistry.load();
  // Bootstrap: fold any legacy static spokes (env / CACTI_SPOKES_CONFIG) into the registry so a
  // single code path drives every watcher. A previously-registered spoke of the same id wins.
  for (const s of config.spokes) {
    if (!spokeRegistry.has(s.id)) {
      await spokeRegistry.register({ ...s, registeredAt: Date.now() });
    }
  }

  // ── Start HTLC relay — watchers are driven by the registry, reconciled at startup and on
  // every runtime registration (no restart, no static startup list). ──────────
  const relay = new HtlcRelay(
    [],
    config.protoPath,
    config.pollIntervalMs,
    config.relayAuthSecret,
    relayStore,
    connectors,
    connectorFactory,
  );
  relay.start(abortController.signal);
  for (const s of spokeRegistry.list()) {
    await relay.addSpoke(s);
  }

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

  // ── Spoke registration (RL-1) ────────────────────────────────────────────
  // NOTE: these endpoints are intentionally outside the /api/v1/relay auth guard
  // so the toolkit registrar (which posts without X-Relay-Auth) can reach them.
  // In a multi-tenant deployment this should be hardened with mutual auth.

  /**
   * POST /api/v1/spokes
   * Register (or re-register) a spoke. Accepts the snake_case body sent by the
   * Go relay registrar. Idempotent: re-registering the same id updates it.
   */
  app.post("/api/v1/spokes", (req: Request, res: Response) => {
    void (async () => {
      try {
        const spoke = normalizeSpokeRegistration((req.body ?? {}) as Record<string, unknown>);
        await spokeRegistry.register(spoke);
        // Start watching immediately — no relay restart. Idempotent for a known spoke.
        await relay.addSpoke(spoke);
        console.log(`[spoke-registry] registered + watching spoke=${spoke.id} rpc=${spoke.besuRpc}`);
        res.status(201).json(spoke);
      } catch (err) {
        if (err instanceof SpokeRegistrationError) {
          res.status(400).json({ error: err.message });
          return;
        }
        console.error("[spoke-registry] register failed:", err);
        res.status(500).json({ error: "failed to register spoke" });
      }
    })();
  });

  /**
   * GET /api/v1/spokes
   * List all registered spokes.
   */
  app.get("/api/v1/spokes", (_req: Request, res: Response) => {
    res.json(spokeRegistry.list());
  });

  /**
   * GET /api/v1/spokes/:id
   * Return a registered spoke (200) or 404. Polled by the toolkit's
   * RelayRegistrar.IsRegistered to confirm registration.
   */
  app.get("/api/v1/spokes/:id", (req: Request, res: Response) => {
    const spoke = spokeRegistry.get(req.params["id"] ?? "");
    if (!spoke) {
      res.status(404).json({ error: "spoke not registered" });
      return;
    }
    res.json(spoke);
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
  // Connectors are used in-process by the relay (getPastLogs/getBlock); no per-connector Cacti
  // web services or Socket.IO stream are exposed.
  const httpServer = http.createServer(app);

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
    httpServer.close(() => process.exit(0));
  };
  process.on("SIGINT", shutdown);
  process.on("SIGTERM", shutdown);
}

main().catch((err: unknown) => {
  console.error("Fatal:", err);
  process.exit(1);
});

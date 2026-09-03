// SPDX-License-Identifier: Apache-2.0

/**
 * HtlcRelay — core interoperability logic for the Cacti HTLC cross-spoke relay.
 *
 * Responsibilities:
 *   1. Use Cacti PluginLedgerConnectorBesu.getPastLogs to fetch HTLC events
 *      from each Besu spoke, with ethers.js Interface for ABI decoding.
 *   2. On LogHTLCClaimed: call SettleHTLC on the counterpart spoke's
 *      payment-orchestrator via gRPC, propagating the revealed secret.
 *   3. Store all observed events in an in-memory ring buffer so the REST API
 *      (consumed by the CactiRelay Go adapter) can poll them by timestamp.
 *
 * The Cacti connector's getPastLogs replaces direct ethers.js JsonRpcProvider
 * log polling, making Cacti the actual ledger abstraction layer. ethers.js is
 * retained only for ABI decoding of the raw EvmLog topics/data fields.
 */

import { ethers } from "ethers";
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";
import type { PluginLedgerConnectorBesu } from "@hyperledger/cactus-plugin-ledger-connector-besu";
import { FXAction, RelayRetryItem, RelayStore } from "./relay-store";

// ---------------------------------------------------------------------------
// Event types
// ---------------------------------------------------------------------------

export interface SettleEvent {
  spoke: string;
  contractId: string;
  secret: string;
  blockNumber: number;
  txHash: string;
  /** Unix epoch milliseconds when this relay process observed the event. */
  timestamp: number;
}

export interface LockEvent {
  spoke: string;
  contractId: string;
  sender: string;
  receiver: string;
  hashLock: string;
  timeLock: number;
  zetoLockRef: string;
  blockNumber: number;
  txHash: string;
  /** Unix epoch milliseconds when this relay process observed the event. */
  timestamp: number;
}

export interface FXProposalEvent {
  spoke: string;
  tradeId: string;
  originator: string;
  counterpartyB: string;
  settlementAgent: string;
  custodian: string;
  beneficiary: string;
  originAmount: string;
  counterAmount: string;
  originCurrency: string;
  counterCurrency: string;
  rate: string;
  expiryDate: number;
  sourceSpokeId: string;
  destSpokeId: string;
  sourceReceiver: string;
  destReceiver: string;
  blockNumber: number;
  txHash: string;
  timestamp: number;
}

export interface FXAcceptanceEvent {
  spoke: string;
  tradeId: string;
  blockNumber: number;
  txHash: string;
  timestamp: number;
}

export interface FXRejectionEvent {
  spoke: string;
  tradeId: string;
  blockNumber: number;
  txHash: string;
  timestamp: number;
}

export interface FXCancellationEvent {
  spoke: string;
  tradeId: string;
  blockNumber: number;
  txHash: string;
  timestamp: number;
}

export interface FXSettlementEvent {
  spoke: string;
  tradeId: string;
  blockNumber: number;
  txHash: string;
  timestamp: number;
}

// ---------------------------------------------------------------------------
// ABI fragments
// ---------------------------------------------------------------------------

const HTLC_ABI = [
  "event LogHTLCLocked(bytes32 indexed contractId, address indexed sender, address indexed receiver, bytes32 hashLock, uint256 timeLock, bytes32 zetoLockRef)",
  "event LogHTLCClaimed(bytes32 indexed contractId, bytes32 secret)",
];

const FX_AGREEMENT_ABI = [
  "event AgreementProposed(bytes32 indexed tradeId, address indexed originator, address indexed counterpartyB, address settlementAgent, address custodian, address beneficiary, uint256 originAmount, uint256 counterAmount, bytes32 originCurrency, bytes32 counterCurrency, uint256 rate, uint256 expiryDate)",
  "event AgreementAccepted(bytes32 indexed tradeId)",
  "event AgreementRejected(bytes32 indexed tradeId)",
];

// ---------------------------------------------------------------------------
// gRPC client helpers
// ---------------------------------------------------------------------------

interface SettleHTLCRequest {
  contract_id: string;
  secret: string;
}

interface SettleHTLCResponse {
  htlc_tx_hash: string;
  zeto_tx_hash: string;
}

interface ProposeFXAgreementGrpcRequest {
  trade_id: string;
  counterparty_b: string;
  originator: string;
  settlement_agent: string;
  custodian: string;
  beneficiary: string;
  origin_amount: string;
  counter_amount: string;
  origin_currency: string;
  counter_currency: string;
  rate: string;
  expiry_date: number;
  source_spoke_id: string;
  dest_spoke_id: string;
  source_receiver: string;
  dest_receiver: string;
  on_behalf: boolean;
}
interface ProposeFXAgreementGrpcResponse { tx_hash: string; trade_id: string; }
interface AcceptFXAgreementGrpcRequest { trade_id: string; on_behalf: boolean; }
interface AcceptFXAgreementGrpcResponse { tx_hash: string; }
interface RejectFXAgreementGrpcRequest { trade_id: string; on_behalf: boolean; }
interface RejectFXAgreementGrpcResponse { tx_hash: string; }
interface CancelFXAgreementGrpcRequest { trade_id: string; }
interface CancelFXAgreementGrpcResponse { tx_hash: string; }
interface SettleFXAgreementGrpcRequest { trade_id: string; }
interface SettleFXAgreementGrpcResponse { tx_hash: string; }

interface PaymentOrchestratorClient extends grpc.Client {
  SettleHTLC(
    req: SettleHTLCRequest,
    callback: (err: grpc.ServiceError | null, resp: SettleHTLCResponse) => void,
  ): void;
  SettleHTLC(
    req: SettleHTLCRequest,
    metadata: grpc.Metadata,
    callback: (err: grpc.ServiceError | null, resp: SettleHTLCResponse) => void,
  ): void;
  SettleHTLC(
    req: SettleHTLCRequest,
    metadata: grpc.Metadata,
    options: grpc.CallOptions,
    callback: (err: grpc.ServiceError | null, resp: SettleHTLCResponse) => void,
  ): void;
  ProposeFXAgreement(
    req: ProposeFXAgreementGrpcRequest,
    metadata: grpc.Metadata,
    options: grpc.CallOptions,
    callback: (err: grpc.ServiceError | null, resp: ProposeFXAgreementGrpcResponse) => void,
  ): void;
  AcceptFXAgreement(
    req: AcceptFXAgreementGrpcRequest,
    metadata: grpc.Metadata,
    options: grpc.CallOptions,
    callback: (err: grpc.ServiceError | null, resp: AcceptFXAgreementGrpcResponse) => void,
  ): void;
  RejectFXAgreement(
    req: RejectFXAgreementGrpcRequest,
    metadata: grpc.Metadata,
    options: grpc.CallOptions,
    callback: (err: grpc.ServiceError | null, resp: RejectFXAgreementGrpcResponse) => void,
  ): void;
  CancelFXAgreement(
    req: CancelFXAgreementGrpcRequest,
    metadata: grpc.Metadata,
    options: grpc.CallOptions,
    callback: (err: grpc.ServiceError | null, resp: CancelFXAgreementGrpcResponse) => void,
  ): void;
  SettleFXAgreement(
    req: SettleFXAgreementGrpcRequest,
    metadata: grpc.Metadata,
    options: grpc.CallOptions,
    callback: (err: grpc.ServiceError | null, resp: SettleFXAgreementGrpcResponse) => void,
  ): void;
}

function createGrpcClient(
  target: string,
  protoPath: string,
): PaymentOrchestratorClient {
  const pkgDef = protoLoader.loadSync(protoPath, {
    keepCase: true,
    longs: String,
    enums: String,
    defaults: true,
    oneofs: true,
  });
  // Package: payment_orchestrator.v1
  // Service: PaymentOrchestratorService
  const proto = grpc.loadPackageDefinition(pkgDef) as Record<
    string,
    Record<string, Record<string, grpc.ServiceClientConstructor>>
  >;
  const ServiceCtor =
    proto["payment_orchestrator"]["v1"]["PaymentOrchestratorService"];
  return new ServiceCtor(
    target,
    grpc.credentials.createInsecure(),
  ) as unknown as PaymentOrchestratorClient;
}

// ---------------------------------------------------------------------------
// SpokeConfig dependency
// ---------------------------------------------------------------------------

export interface SpokeDep {
  id: string;
  besuRpc: string;
  besuWs: string;
  htlcAddress: string;
  internalApiUrl: string;
  grpcEndpoint: string;
}

// ---------------------------------------------------------------------------
// HtlcRelay
// ---------------------------------------------------------------------------

/** Maximum number of events retained per category. Acts as a circular buffer. */
const MAX_EVENTS = 10_000;

/**
 * Max blocks scanned per getPastLogs cycle. A long catch-up (relay down for many blocks) is
 * walked one bounded chunk per poll so a single request cannot exceed the node's log-range
 * limit or stall the loop (finding R2-H-11: range chunking for long catch-ups).
 */
const MAX_BLOCK_RANGE = 5_000;

export class HtlcRelay {
  // Lock events are also kept in a small in-memory ring purely for resolveCounterpart's
  // same-process hashLock lookup. The durable, seq-numbered copy served to the Go poller lives
  // in the RelayStore journal (finding R2-H-11) — this ring is not the serving source of truth.
  private readonly lockEvents: LockEvent[] = [];
  private readonly fxProposalEvents: FXProposalEvent[] = [];
  private readonly fxAcceptanceEvents: FXAcceptanceEvent[] = [];
  private readonly fxRejectionEvents: FXRejectionEvent[] = [];
  private readonly fxCancellationEvents: FXCancellationEvent[] = [];
  private readonly fxSettlementEvents: FXSettlementEvent[] = [];
  // Feedback-loop and replay prevention are now PERSISTED in the RelayStore (finding R2-H-11),
  // not in an in-memory Set that a restart would clear:
  //   - `htlc-evt:${spoke}:${txHash}:${logIndex}` — this observed claim was already forwarded
  //     (survives a crash between forwarding and the watermark advance, so no duplicate settle).
  //   - `htlc-settled:${destSpoke}:${destContractId}` — this contract was settled BY this relay,
  //     so the resulting LogHTLCClaimed echo on the destination must not be forwarded back.
  // Echo prevention is keyed on the destination contract id, not the secret: a preimage may be
  // legitimately reused across legs, so a secret is not a unique key (review R2-H-11).
  /** Tracks trade IDs already forwarded cross-spoke. Value = timestamp (ms). */
  private readonly forwardedTradeIds = new Map<string, number>();
  /** gRPC client per spoke, keyed by spoke.id. Created on add and reused for all settlements. */
  private readonly grpcClients = new Map<string, PaymentOrchestratorClient>();
  /** Spokes currently being watched (a pollSpoke loop is running), keyed by id. */
  private readonly watching = new Set<string>();
  /** The lifecycle signal, captured in start() and reused by runtime addSpoke() calls. */
  private signal?: AbortSignal;

  constructor(
    private readonly spokes: SpokeDep[],
    private readonly protoPath: string,
    private readonly pollIntervalMs: number,
    private readonly relayAuthSecret: string,
    private readonly relayStore: RelayStore,
    private readonly cactiConnectors: Map<string, PluginLedgerConnectorBesu>,
    // connectorFactory (re)creates a Besu connector for a spoke: used to attach a connector when
    // a spoke is registered at runtime, and to reconnect after WS failures. Optional so unit
    // tests that never poll can omit it.
    private readonly connectorFactory?: (spoke: SpokeDep) => Promise<PluginLedgerConnectorBesu>,
    private readonly log: Pick<Console, "info" | "warn" | "error"> = console,
  ) {}

  // ── Public accessors (used by REST API) ────────────────────────────────

  // Lock/settle events are served from the durable seq journal, not the in-memory ring: the
  // caller (Go poller) passes the last seq it processed and receives events with a greater seq.
  // The seq is stable across relay restarts, so the poller's persisted cursor composes exactly
  // with the relay and never re-delivers or loses events (finding R2-H-11).
  getSettleEvents(sinceSeq = 0): Record<string, unknown>[] {
    return this.relayStore.getEventsSince("settle", sinceSeq);
  }

  getLockEvents(sinceSeq = 0): Record<string, unknown>[] {
    return this.relayStore.getEventsSince("lock", sinceSeq);
  }

  getFXProposalEvents(sinceMs = 0): FXProposalEvent[] {
    return this.fxProposalEvents.filter((e) => e.timestamp >= sinceMs);
  }

  getFXAcceptanceEvents(sinceMs = 0): FXAcceptanceEvent[] {
    return this.fxAcceptanceEvents.filter((e) => e.timestamp >= sinceMs);
  }

  getFXRejectionEvents(sinceMs = 0): FXRejectionEvent[] {
    return this.fxRejectionEvents.filter((e) => e.timestamp >= sinceMs);
  }

  getFXCancellationEvents(sinceMs = 0): FXCancellationEvent[] {
    return this.fxCancellationEvents.filter((e) => e.timestamp >= sinceMs);
  }

  getFXSettlementEvents(sinceMs = 0): FXSettlementEvent[] {
    return this.fxSettlementEvents.filter((e) => e.timestamp >= sinceMs);
  }

  // ── Lifecycle ──────────────────────────────────────────────────────────

  /**
   * Start the relay. Captures the lifecycle AbortSignal (cancel it to shut down) and begins
   * watching any spokes supplied at construction. Additional spokes are attached at runtime via
   * addSpoke() as they register — the watcher set is driven by the dynamic registry, not a fixed
   * startup list.
   */
  start(signal: AbortSignal): void {
    this.signal = signal;
    signal.addEventListener("abort", () => {
      for (const c of this.grpcClients.values()) c.close();
    }, { once: true });
    for (const spoke of this.spokes) {
      void this.addSpoke(spoke);
    }
  }

  /**
   * addSpoke begins watching a spoke: it ensures a Besu connector exists (creating one via
   * connectorFactory when absent), creates the spoke's gRPC client, and launches its poll loop.
   * Idempotent — a spoke already being watched is ignored (re-registration does not restart it).
   * Safe to call before or after start(); requires start() to have captured the signal.
   */
  async addSpoke(spoke: SpokeDep): Promise<void> {
    if (this.watching.has(spoke.id)) {
      return;
    }
    if (!this.signal) {
      this.log.error(`[${spoke.id}] addSpoke called before start() — ignored`);
      return;
    }
    if (!this.cactiConnectors.has(spoke.id)) {
      if (!this.connectorFactory) {
        this.log.error(`[${spoke.id}] no Cacti connector and no connectorFactory — cannot watch`);
        return;
      }
      try {
        this.cactiConnectors.set(spoke.id, await this.connectorFactory(spoke));
      } catch (err) {
        this.log.error(`[${spoke.id}] connector init failed, will not watch: ${String(err)}`);
        return;
      }
    }
    this.watching.add(spoke.id);
    this.grpcClients.set(spoke.id, createGrpcClient(spoke.grpcEndpoint, this.protoPath));
    this.log.info(`[${spoke.id}] watcher started rpc=${spoke.besuRpc} htlc=${spoke.htlcAddress} internalApi=${spoke.internalApiUrl}`);
    this.pollSpoke(spoke, this.signal).catch((err: unknown) => {
      this.watching.delete(spoke.id);
      this.log.error(`[${spoke.id}] fatal poll error: ${String(err)}`);
    });
  }

  // ── Private helpers ────────────────────────────────────────────────────

  private async pollSpoke(
    spoke: SpokeDep,
    signal: AbortSignal,
  ): Promise<void> {
    let connector = this.cactiConnectors.get(spoke.id);
    if (!connector) {
      this.log.error(`[${spoke.id}] no Cacti connector registered — cannot poll`);
      return;
    }
    // Consecutive poll-cycle failures; after RECONNECT_AFTER we rebuild the Besu connector to
    // recover from a dropped WebSocket ("connection not open on send()"), which never self-heals.
    let failures = 0;
    const RECONNECT_AFTER = 3;

    // ethers Interface used only for ABI decoding of raw EvmLog topics/data.
    const iface = new ethers.Interface(HTLC_ABI);

    // Compute event topic hashes for log filtering via Cacti getPastLogs.
    const topicLocked = ethers.id("LogHTLCLocked(bytes32,address,address,bytes32,uint256,bytes32)");
    const topicClaimed = ethers.id("LogHTLCClaimed(bytes32,bytes32)");

    // Determine the starting block. Resume from the persisted watermark so events
    // mined while the relay was down are not skipped (finding R2-H-11); only fall
    // back to the chain head on a first-ever start with no recorded watermark.
    let fromBlock: number;
    const persisted = this.relayStore.getWatermark(spoke.id);
    if (persisted !== undefined) {
      fromBlock = persisted + 1;
      this.log.info(`[${spoke.id}] relay resuming from persisted watermark block ${fromBlock}`);
    } else {
      try {
        const blockResp = await connector.getBlock({ blockHashOrBlockNumber: "latest" });
        fromBlock = typeof blockResp.block === "object" && blockResp.block !== null
          ? Number((blockResp.block as Record<string, unknown>)["number"] ?? 0)
          : 0;
      } catch (err) {
        this.log.error(`[${spoke.id}] cannot get current block via Cacti: ${String(err)}`);
        fromBlock = 0;
      }
      this.log.info(`[${spoke.id}] relay started at block ${fromBlock} (via Cacti connector, no persisted watermark)`);
    }

    // Detect a chain reset (fresh genesis under a stale on-volume watermark). Without this the
    // relay would sit forever with head below watermark, silently forwarding nothing (R2-H-11).
    fromBlock = await this.reconcileSpokeChain(connector, spoke, fromBlock);

    while (!signal.aborted) {
      await sleep(this.pollIntervalMs);
      if (signal.aborted) break;

      try {
        // Get latest block number via Cacti connector.
        const latestResp = await connector.getBlock({ blockHashOrBlockNumber: "latest" });
        const latestBlock = typeof latestResp.block === "object" && latestResp.block !== null
          ? Number((latestResp.block as Record<string, unknown>)["number"] ?? 0)
          : 0;
        if (latestBlock < fromBlock) {
          // Head is behind our resume point: either the chain has not produced new blocks yet, or
          // (after a reset the genesis guard did not catch) the watermark is ahead of the chain.
          // Never a silent no-op — warn so a stuck relay is visible (constitution: no silent failures).
          this.log.warn(
            `[${spoke.id}] chain head ${latestBlock} is behind resume block ${fromBlock} — waiting (no events forwarded)`,
          );
          continue;
        }
        // Walk at most MAX_BLOCK_RANGE blocks this cycle so a long catch-up is chunked.
        const toBlock = Math.min(fromBlock + MAX_BLOCK_RANGE - 1, latestBlock);

        // Fetch logs via Cacti PluginLedgerConnectorBesu.getPastLogs — the
        // core integration point that replaces direct ethers.js provider usage.
        const [lockedResp, claimedResp] = await Promise.all([
          connector.getPastLogs({
            address: spoke.htlcAddress,
            fromBlock,
            toBlock,
            topics: [[topicLocked]],
          }),
          connector.getPastLogs({
            address: spoke.htlcAddress,
            fromBlock,
            toBlock,
            topics: [[topicClaimed]],
          }),
        ]);

        // Process LogHTLCLocked events — decode via ethers Interface.
        for (const raw of lockedResp.logs) {
          try {
            const parsed = iface.parseLog({ topics: raw.topics, data: raw.data });
            if (!parsed) continue;
            const logIndex = Number((raw as unknown as Record<string, unknown>)["logIndex"] ?? 0);
            const evt: LockEvent = {
              spoke: spoke.id,
              contractId: strip0x(parsed.args[0] as string),
              sender: parsed.args[1] as string,
              receiver: parsed.args[2] as string,
              hashLock: strip0x(parsed.args[3] as string),
              timeLock: Number(parsed.args[4]),
              zetoLockRef: strip0x(parsed.args[5] as string),
              blockNumber: raw.blockNumber,
              txHash: raw.transactionHash,
              timestamp: Date.now(),
            };
            // Ring for resolveCounterpart's in-process lookup; journal (deduped by chain id) is the
            // durable, seq-stable copy served to the Go poller (finding R2-H-11).
            pushRing(this.lockEvents, evt, MAX_EVENTS);
            await this.relayStore.appendEvent(
              "lock",
              `${spoke.id}:${raw.transactionHash}:${logIndex}`,
              evt as unknown as Record<string, unknown>,
            );
            this.log.info(
              `[${spoke.id}] LogHTLCLocked contractId=${evt.contractId} block=${evt.blockNumber}`,
            );
          } catch (decodeErr) {
            this.log.warn(`[${spoke.id}] failed to decode LogHTLCLocked: ${String(decodeErr)}`);
          }
        }

        // Process LogHTLCClaimed events — decode and forward settlement. A settlement that fails
        // to deliver must NOT let the watermark advance past its block, so the event is retried
        // rather than silently dropped (finding R2-H-11). We track the lowest failed block here.
        let minFailedBlock = Number.POSITIVE_INFINITY;
        for (const raw of claimedResp.logs) {
          try {
            const parsed = iface.parseLog({ topics: raw.topics, data: raw.data });
            if (!parsed) continue;
            const contractId = strip0x(parsed.args[0] as string);
            const secret = strip0x(parsed.args[1] as string);
            const logIndex = Number((raw as unknown as Record<string, unknown>)["logIndex"] ?? 0);
            const evt: SettleEvent = {
              spoke: spoke.id,
              contractId,
              secret,
              blockNumber: raw.blockNumber,
              txHash: raw.transactionHash,
              timestamp: Date.now(),
            };
            // Durable, seq-stable journal entry served to the Go poller (deduped by chain id so a
            // re-decode after restart reuses the same seq — no re-delivery) (finding R2-H-11).
            await this.relayStore.appendEvent(
              "settle",
              `${spoke.id}:${raw.transactionHash}:${logIndex}`,
              evt as unknown as Record<string, unknown>,
            );
            this.log.info(
              `[${spoke.id}] LogHTLCClaimed contractId=${contractId} block=${raw.blockNumber} tx=${raw.transactionHash}`,
            );

            // Per-event replay guard (survives restart): this exact observed claim was already
            // forwarded. Keyed on txHash:logIndex — the stable event identity, not the secret.
            const evtKey = `htlc-evt:${spoke.id}:${raw.transactionHash}:${logIndex}`;
            if (this.relayStore.hasDelivered(evtKey)) {
              continue;
            }

            // Echo guard: a claim for a contract THIS relay settled is the settlement's own event
            // bouncing back; do not forward it to the origin (already claimed there).
            if (this.relayStore.hasDelivered(`htlc-settled:${spoke.id}:${contractId}`)) {
              this.log.info(
                `[${spoke.id}] skipping echo claim for relay-settled contractId=${contractId}`,
              );
              await this.relayStore.markDelivered(evtKey);
              continue;
            }

            const resolved = this.resolveCounterpart(spoke.id, contractId);
            if (!resolved) {
              // Counterpart lock not observed (yet). Do not mark delivered — a later cycle that
              // re-scans this block (e.g. after another failure) can still resolve and forward it.
              this.log.warn(`[${spoke.id}] skipping settlement — could not resolve counterpart for contractId=${contractId}`);
              continue;
            }
            const destClient = this.grpcClients.get(resolved.destSpokeId);
            if (!destClient) {
              this.log.error(`[${spoke.id}] settlement skipped: dest_spoke_id "${resolved.destSpokeId}" not in registry`);
              continue;
            }
            const settled = await this.settleOnCounterpart(destClient, spoke.id, resolved.contractId, secret);
            if (settled) {
              await this.relayStore.markDelivered(evtKey);
              await this.relayStore.markDelivered(`htlc-settled:${resolved.destSpokeId}:${resolved.contractId}`);
            } else {
              // Hold the watermark at/below this block so the failed settlement is retried.
              minFailedBlock = Math.min(minFailedBlock, raw.blockNumber);
            }
          } catch (decodeErr) {
            this.log.warn(`[${spoke.id}] failed to decode LogHTLCClaimed: ${String(decodeErr)}`);
          }
        }

        // ── FX Agreement polling (REST-based, no on-chain contract) ─────
        this.pruneForwardedTradeIds();
        await this.processDueRetriesForSpoke(spoke.id);
        await this.pollFXAgreementsRest(spoke);

        const stats = this.relayStore.getRetryStats();
        if (stats.pending > 0) {
          this.log.info(
            `[relay] retry-stats pending=${stats.pending} max_lag_ms=${stats.maxLagMs}`,
          );
        }

        // Advance the watermark only through the blocks whose settlements actually delivered.
        // If a settlement failed at block N, stop at N-1 so [N, toBlock] is retried next cycle
        // instead of being skipped (advance-after-delivery, not after-fetch — finding R2-H-11).
        const deliveredThrough = minFailedBlock === Number.POSITIVE_INFINITY
          ? toBlock
          : minFailedBlock - 1;
        await this.relayStore.setWatermark(spoke.id, deliveredThrough);
        fromBlock = deliveredThrough + 1;
        failures = 0;
      } catch (err) {
        this.log.warn(`[${spoke.id}] poll cycle error: ${String(err)}`);
        failures++;
        if (failures >= RECONNECT_AFTER && this.connectorFactory) {
          this.log.warn(`[${spoke.id}] reconnecting Besu connector after ${failures} consecutive failures`);
          try {
            const fresh = await this.connectorFactory(spoke);
            try { await connector.shutdown(); } catch { /* best-effort */ }
            connector = fresh;
            this.cactiConnectors.set(spoke.id, fresh);
            failures = 0;
            this.log.info(`[${spoke.id}] Besu connector reconnected`);
          } catch (reErr) {
            this.log.error(`[${spoke.id}] reconnect failed: ${String(reErr)}`);
          }
        }
      }
    }

    this.watching.delete(spoke.id);
    this.log.info(`[${spoke.id}] relay stopped`);
  }

  /**
   * Guard against a chain reset that leaves a stale watermark on the relay volume. Record the
   * spoke chain's genesis-block hash the first time; on a later boot, if the genesis hash differs
   * the chain was reset — rewind the watermark to 0, drop this spoke's dedup keys, and re-sync
   * from the new genesis instead of stalling with head permanently below watermark (R2-H-11).
   * Returns the block to resume from (unchanged unless a reset was detected).
   */
  private async reconcileSpokeChain(
    connector: PluginLedgerConnectorBesu,
    spoke: SpokeDep,
    fromBlock: number,
  ): Promise<number> {
    let genesisHash: string;
    try {
      const resp = await connector.getBlock({ blockHashOrBlockNumber: 0 });
      const block = resp.block as Record<string, unknown> | undefined;
      genesisHash = String(block?.["hash"] ?? "");
    } catch (err) {
      this.log.warn(`[${spoke.id}] could not read genesis hash for chain-reset guard: ${String(err)}`);
      return fromBlock;
    }
    if (!genesisHash) return fromBlock;

    const known = this.relayStore.getMeta(spoke.id);
    if (known === undefined) {
      await this.relayStore.setMeta(spoke.id, genesisHash);
      return fromBlock;
    }
    if (known !== genesisHash) {
      this.log.warn(
        `[${spoke.id}] chain genesis changed (${known} -> ${genesisHash}) — chain was reset; ` +
        `rewinding watermark to 0 and re-syncing`,
      );
      await this.relayStore.resetSpokeChain(spoke.id, 0, genesisHash);
      return 0;
    }
    return fromBlock;
  }

  private resolveCounterpart(
    spokeName: string,
    contractId: string,
  ): { destSpokeId: string; contractId: string } | undefined {
    // Walk backwards so we pick the most recent matching event.
    let sourceLock: LockEvent | undefined;
    for (let i = this.lockEvents.length - 1; i >= 0; i--) {
      const l = this.lockEvents[i];
      if (l.spoke === spokeName && l.contractId === contractId) {
        sourceLock = l;
        break;
      }
    }
    if (!sourceLock) {
      this.log.warn(
        `[${spokeName}] resolveCounterpart: source lock not found for contractId=${contractId}`,
      );
      return undefined;
    }

    let counterpartLock: LockEvent | undefined;
    for (let i = this.lockEvents.length - 1; i >= 0; i--) {
      const l = this.lockEvents[i];
      if (l.spoke !== spokeName && l.hashLock === sourceLock.hashLock) {
        counterpartLock = l;
        break;
      }
    }
    if (!counterpartLock) {
      this.log.warn(
        `[${spokeName}] resolveCounterpart: no counterpart lock found for hashLock=${sourceLock.hashLock}`,
      );
      return undefined;
    }

    this.log.info(
      `[${spokeName}] resolveCounterpart: ${contractId} → ${counterpartLock.contractId} (destSpokeId=${counterpartLock.spoke})`,
    );
    return { destSpokeId: counterpartLock.spoke, contractId: counterpartLock.contractId };
  }

  /**
   * Settle on the counterpart spoke. Resolves true only when the gRPC call succeeded; a failure
   * resolves false so the caller holds the watermark and retries instead of marking the event
   * delivered (finding R2-H-11: the watermark must mean delivered, not merely attempted).
   */
  private settleOnCounterpart(
    client: PaymentOrchestratorClient,
    spokeName: string,
    contractId: string,
    secret: string,
  ): Promise<boolean> {
    return new Promise((resolve) => {
      const deadline = new Date(Date.now() + 30_000);
      client.SettleHTLC(
        { contract_id: contractId, secret },
        new grpc.Metadata(),
        { deadline },
        (err: grpc.ServiceError | null, resp: SettleHTLCResponse) => {
          if (err) {
            this.log.error(
              `[${spokeName}] SettleHTLC gRPC failed contractId=${contractId}: ${err.message}`,
            );
            resolve(false);
          } else {
            this.log.info(
              `[${spokeName}] counterpart settled contractId=${contractId} htlcTx=${resp?.htlc_tx_hash} zetoTx=${resp?.zeto_tx_hash}`,
            );
            resolve(true);
          }
        },
      );
    });
  }

  private pruneForwardedTradeIds(): void {
    const cutoff = Date.now() - 7 * 24 * 60 * 60 * 1000;
    for (const [id, ts] of this.forwardedTradeIds) {
      if (ts < cutoff) this.forwardedTradeIds.delete(id);
    }
  }

  private async pollFXAgreementsRest(
    spoke: SpokeDep,
  ): Promise<void> {
    const url = `${spoke.internalApiUrl}/internal/v1/payments/fx/agreements`;
    let body: { agreements?: unknown[] };
    try {
      const res = await fetch(url, {
        headers: {
          "X-Relay-Auth": this.relayAuthSecret,
        },
      });
      if (!res.ok) {
        this.log.warn(`[${spoke.id}] FX REST poll failed: ${res.status}`);
        return;
      }
      body = await res.json() as { agreements?: unknown[] };
    } catch (err) {
      this.log.warn(`[${spoke.id}] FX REST poll error: ${String(err)}`);
      return;
    }

    const agreements = body.agreements ?? [];
    for (const raw of agreements) {
      const a = raw as Record<string, unknown>;
      const tradeId = a["trade_id"] as string;
      const state = a["state"] as string;
      if (!tradeId || !state) continue;

      if (state === "FX_STATE_PROPOSED" && !(await this.wasForwarded(`propose:${tradeId}`))) {
        const destSpokeId = (a["dest_spoke_id"] as string) ?? "";
        const sourceSpokeId = (a["source_spoke_id"] as string) ?? "";
        if (!sourceSpokeId || !destSpokeId) {
          this.log.error(`[${spoke.id}] FX REST: skipping proposal tradeId=${tradeId} — missing source_spoke_id or dest_spoke_id`);
          continue;
        }
        const client = this.grpcClients.get(destSpokeId);
        if (!client) {
          this.log.error(`[${spoke.id}] FX REST: skipping proposal tradeId=${tradeId} — dest_spoke_id "${destSpokeId}" not in registry`);
          continue;
        }
        const evt: FXProposalEvent = {
          spoke: spoke.id,
          tradeId,
          originator: (a["originator"] as string) ?? "",
          counterpartyB: a["counterparty_b"] as string,
          settlementAgent: a["settlement_agent"] as string,
          custodian: a["custodian"] as string,
          beneficiary: a["beneficiary"] as string,
          originAmount: a["origin_amount"] as string,
          counterAmount: a["counter_amount"] as string,
          originCurrency: a["origin_currency"] as string,
          counterCurrency: a["counter_currency"] as string,
          rate: a["rate"] as string,
          expiryDate: a["expiry_date"] as number,
          sourceSpokeId,
          destSpokeId,
          sourceReceiver: (a["source_receiver"] as string) ?? "",
          destReceiver: (a["dest_receiver"] as string) ?? "",
          blockNumber: 0,
          txHash: "",
          timestamp: Date.now(),
        };
        pushRing(this.fxProposalEvents, evt, MAX_EVENTS);
        this.log.info(`[${spoke.id}] FX REST: forwarding proposal tradeId=${tradeId} sourceSpokeId=${sourceSpokeId} destSpokeId=${destSpokeId}`);
        const payload = { ...(evt as unknown as Record<string, unknown>), destSpokeId };
        await this.tryForwardFXAction("propose", spoke.id, tradeId, client, payload);

      } else if (state === "FX_STATE_ACCEPTED" && !(await this.wasForwarded(`accept:${tradeId}`))) {
        const sourceSpokeId = a["source_spoke_id"] as string;
        const client = this.grpcClients.get(sourceSpokeId);
        if (!client) {
          this.log.error(`[${spoke.id}] FX REST: skipping acceptance tradeId=${tradeId} — source_spoke_id "${sourceSpokeId}" not in registry`);
          continue;
        }
        const evt: FXAcceptanceEvent = {
          spoke: spoke.id,
          tradeId,
          blockNumber: 0,
          txHash: "",
          timestamp: Date.now(),
        };
        pushRing(this.fxAcceptanceEvents, evt, MAX_EVENTS);
        this.log.info(`[${spoke.id}] FX REST: forwarding acceptance tradeId=${tradeId}`);
        await this.tryForwardFXAction("accept", spoke.id, tradeId, client, { destSpokeId: sourceSpokeId });

      } else if (state === "FX_STATE_REJECTED" && !(await this.wasForwarded(`reject:${tradeId}`))) {
        const sourceSpokeId = a["source_spoke_id"] as string;
        const client = this.grpcClients.get(sourceSpokeId);
        if (!client) {
          this.log.error(`[${spoke.id}] FX REST: skipping rejection tradeId=${tradeId} — source_spoke_id "${sourceSpokeId}" not in registry`);
          continue;
        }
        const evt: FXRejectionEvent = {
          spoke: spoke.id,
          tradeId,
          blockNumber: 0,
          txHash: "",
          timestamp: Date.now(),
        };
        pushRing(this.fxRejectionEvents, evt, MAX_EVENTS);
        this.log.info(`[${spoke.id}] FX REST: forwarding rejection tradeId=${tradeId}`);
        await this.tryForwardFXAction("reject", spoke.id, tradeId, client, { destSpokeId: sourceSpokeId });

      } else if (state === "FX_STATE_CANCELLED" && !(await this.wasForwarded(`cancel:${tradeId}`))) {
        const sourceSpokeId = a["source_spoke_id"] as string;
        const client = this.grpcClients.get(sourceSpokeId);
        if (!client) {
          this.log.error(`[${spoke.id}] FX REST: skipping cancellation tradeId=${tradeId} — source_spoke_id "${sourceSpokeId}" not in registry`);
          continue;
        }
        const evt: FXCancellationEvent = {
          spoke: spoke.id,
          tradeId,
          blockNumber: 0,
          txHash: "",
          timestamp: Date.now(),
        };
        pushRing(this.fxCancellationEvents, evt, MAX_EVENTS);
        this.log.info(`[${spoke.id}] FX REST: forwarding cancellation tradeId=${tradeId}`);
        await this.tryForwardFXAction("cancel", spoke.id, tradeId, client, { destSpokeId: sourceSpokeId });

      } else if (state === "FX_STATE_SETTLED" && !(await this.wasForwarded(`settle:${tradeId}`))) {
        const sourceSpokeId = a["source_spoke_id"] as string;
        const client = this.grpcClients.get(sourceSpokeId);
        if (!client) {
          this.log.error(`[${spoke.id}] FX REST: skipping settlement tradeId=${tradeId} — source_spoke_id "${sourceSpokeId}" not in registry`);
          continue;
        }
        const evt: FXSettlementEvent = {
          spoke: spoke.id,
          tradeId,
          blockNumber: 0,
          txHash: "",
          timestamp: Date.now(),
        };
        pushRing(this.fxSettlementEvents, evt, MAX_EVENTS);
        this.log.info(`[${spoke.id}] FX REST: forwarding settlement tradeId=${tradeId}`);
        await this.tryForwardFXAction("settle", spoke.id, tradeId, client, { destSpokeId: sourceSpokeId });
      }
    }
  }

  private async processDueRetriesForSpoke(
    spokeName: string,
  ): Promise<void> {
    const due = await this.relayStore.getDueRetries(spokeName);
    for (const item of due) {
      const destSpokeId = item.payload?.["destSpokeId"] as string | undefined;
      if (!destSpokeId) {
        this.log.error(`[${spokeName}] retry skipped: no destSpokeId in payload for tradeId=${item.tradeId}`);
        continue;
      }
      const client = this.grpcClients.get(destSpokeId);
      if (!client) {
        this.log.error(`[${spokeName}] retry skipped: dest_spoke_id "${destSpokeId}" not in registry for tradeId=${item.tradeId}`);
        continue;
      }
      await this.tryForwardFXAction(item.action, spokeName, item.tradeId, client, item.payload, item);
    }
  }

  private async wasForwarded(key: string): Promise<boolean> {
    return this.forwardedTradeIds.has(key) || this.relayStore.hasDelivered(key);
  }

  private async tryForwardFXAction(
    action: FXAction,
    spokeName: string,
    tradeId: string,
    client: PaymentOrchestratorClient,
    payload?: Record<string, unknown>,
    retryItem?: RelayRetryItem,
  ): Promise<void> {
    const key = `${action}:${tradeId}`;
    if (await this.wasForwarded(key)) {
      return;
    }

    let ok = false;
    switch (action) {
      case "propose":
        ok = await this.proposeOnCounterpart(client, spokeName, payload as unknown as FXProposalEvent);
        break;
      case "accept":
        ok = await this.acceptOnCounterpart(client, spokeName, tradeId);
        break;
      case "reject":
        ok = await this.rejectOnCounterpart(client, spokeName, tradeId);
        break;
      case "cancel":
        ok = await this.cancelOnCounterpart(client, spokeName, tradeId);
        break;
      case "settle":
        ok = await this.settleAgreementOnCounterpart(client, spokeName, tradeId);
        break;
      default:
        this.log.warn(`[${spokeName}] unsupported retry action=${String(action)} tradeId=${tradeId}`);
    }

    if (ok) {
      this.forwardedTradeIds.set(key, Date.now());
      await this.relayStore.markDelivered(key);
      return;
    }

    const reason = retryItem?.lastError ?? `forward ${action} failed`;
    await this.relayStore.scheduleRetry(key, action, spokeName, tradeId, reason, payload);
  }

  private proposeOnCounterpart(
    client: PaymentOrchestratorClient,
    spokeName: string,
    event: FXProposalEvent,
  ): Promise<boolean> {
    return new Promise((resolve) => {
      const deadline = new Date(Date.now() + 30_000);
      client.ProposeFXAgreement(
        {
          trade_id: event.tradeId,
          counterparty_b: event.counterpartyB,
          originator: event.originator,
          settlement_agent: event.settlementAgent,
          custodian: event.custodian,
          beneficiary: event.beneficiary,
          origin_amount: event.originAmount,
          counter_amount: event.counterAmount,
          origin_currency: event.originCurrency,
          counter_currency: event.counterCurrency,
          rate: event.rate,
          expiry_date: event.expiryDate,
          source_spoke_id: event.sourceSpokeId,
          dest_spoke_id: event.destSpokeId,
          source_receiver: event.sourceReceiver,
          dest_receiver: event.destReceiver,
          on_behalf: true,
        },
        new grpc.Metadata(),
        { deadline },
        (err: grpc.ServiceError | null, resp: ProposeFXAgreementGrpcResponse) => {
          if (err) {
            if (isFXAlreadyInState(err, "PROPOSED", "ACCEPTED", "SETTLED")) {
              this.log.warn(
                `[${spokeName}] ProposeFXAgreement already proposed tradeId=${event.tradeId}, skipping`,
              );
              resolve(true);
            } else {
              this.log.error(
                `[${spokeName}] ProposeFXAgreement gRPC failed tradeId=${event.tradeId}: ${err.message}`,
              );
              resolve(false);
            }
          } else {
            this.log.info(
              `[${spokeName}] counterpart proposed tradeId=${event.tradeId} tx=${resp?.tx_hash}`,
            );
            resolve(true);
          }
        },
      );
    });
  }

  private acceptOnCounterpart(
    client: PaymentOrchestratorClient,
    spokeName: string,
    tradeId: string,
  ): Promise<boolean> {
    return new Promise((resolve) => {
      const deadline = new Date(Date.now() + 30_000);
      client.AcceptFXAgreement(
        { trade_id: tradeId, on_behalf: true },
        new grpc.Metadata(),
        { deadline },
        (err: grpc.ServiceError | null, resp: AcceptFXAgreementGrpcResponse) => {
          if (err) {
            if (isFXAlreadyInState(err, "ACCEPTED")) {
              this.log.warn(
                `[${spokeName}] AcceptFXAgreement already accepted tradeId=${tradeId}, skipping`,
              );
              resolve(true);
            } else {
              this.log.error(
                `[${spokeName}] AcceptFXAgreement gRPC failed tradeId=${tradeId}: ${err.message}`,
              );
              resolve(false);
            }
          } else {
            this.log.info(
              `[${spokeName}] counterpart accepted tradeId=${tradeId} tx=${resp?.tx_hash}`,
            );
            resolve(true);
          }
        },
      );
    });
  }

  private rejectOnCounterpart(
    client: PaymentOrchestratorClient,
    spokeName: string,
    tradeId: string,
  ): Promise<boolean> {
    return new Promise((resolve) => {
      const deadline = new Date(Date.now() + 30_000);
      client.RejectFXAgreement(
        { trade_id: tradeId, on_behalf: true },
        new grpc.Metadata(),
        { deadline },
        (err: grpc.ServiceError | null, resp: RejectFXAgreementGrpcResponse) => {
          if (err) {
            if (isFXAlreadyInState(err, "REJECTED", "CANCELLED")) {
              this.log.warn(
                `[${spokeName}] RejectFXAgreement already rejected tradeId=${tradeId}, skipping`,
              );
              resolve(true);
            } else {
              this.log.error(
                `[${spokeName}] RejectFXAgreement gRPC failed tradeId=${tradeId}: ${err.message}`,
              );
              resolve(false);
            }
          } else {
            this.log.info(
              `[${spokeName}] counterpart rejected tradeId=${tradeId} tx=${resp?.tx_hash}`,
            );
            resolve(true);
          }
        },
      );
    });
  }

  private cancelOnCounterpart(
    client: PaymentOrchestratorClient,
    spokeName: string,
    tradeId: string,
  ): Promise<boolean> {
    return new Promise((resolve) => {
      const deadline = new Date(Date.now() + 30_000);
      client.CancelFXAgreement(
        { trade_id: tradeId },
        new grpc.Metadata(),
        { deadline },
        (err: grpc.ServiceError | null, resp: CancelFXAgreementGrpcResponse) => {
          if (err) {
            if (isFXAlreadyInState(err, "CANCELLED", "REJECTED")) {
              this.log.warn(
                `[${spokeName}] CancelFXAgreement already cancelled tradeId=${tradeId}, skipping`,
              );
              resolve(true);
            } else {
              this.log.error(
                `[${spokeName}] CancelFXAgreement gRPC failed tradeId=${tradeId}: ${err.message}`,
              );
              resolve(false);
            }
          } else {
            this.log.info(
              `[${spokeName}] counterpart cancelled tradeId=${tradeId} tx=${resp?.tx_hash}`,
            );
            resolve(true);
          }
        },
      );
    });
  }

  private settleAgreementOnCounterpart(
    client: PaymentOrchestratorClient,
    spokeName: string,
    tradeId: string,
  ): Promise<boolean> {
    return new Promise((resolve) => {
      const deadline = new Date(Date.now() + 30_000);
      client.SettleFXAgreement(
        { trade_id: tradeId },
        new grpc.Metadata(),
        { deadline },
        (err: grpc.ServiceError | null, resp: SettleFXAgreementGrpcResponse) => {
          if (err) {
            if (isFXAlreadyInState(err, "SETTLED", "FINALIZED")) {
              this.log.warn(
                `[${spokeName}] SettleFXAgreement already settled tradeId=${tradeId}, skipping`,
              );
              resolve(true);
            } else {
              this.log.error(
                `[${spokeName}] SettleFXAgreement gRPC failed tradeId=${tradeId}: ${err.message}`,
              );
              resolve(false);
            }
          } else {
            this.log.info(
              `[${spokeName}] counterpart settled tradeId=${tradeId} tx=${resp?.tx_hash}`,
            );
            resolve(true);
          }
        },
      );
    });
  }
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Returns true when a gRPC FAILED_PRECONDITION error indicates the FX agreement
 * is already in one of the given terminal states — meaning the action was already
 * applied and retrying it would always fail. Callers should treat this as success.
 */
function isFXAlreadyInState(err: grpc.ServiceError, ...states: string[]): boolean {
  if (err.code !== grpc.status.FAILED_PRECONDITION) return false;
  const upper = err.message.toUpperCase();
  return states.some((s) => upper.includes(`IS IN STATE ${s.toUpperCase()}`));
}

function strip0x(hex: string): string {
  return hex.startsWith("0x") ? hex.slice(2) : hex;
}

function pushRing<T>(arr: T[], item: T, max: number): void {
  arr.push(item);
  if (arr.length > max) arr.shift();
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

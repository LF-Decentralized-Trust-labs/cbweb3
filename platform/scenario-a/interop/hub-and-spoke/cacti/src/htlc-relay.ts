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
  spokeAReceiver: string;
  spokenBReceiver: string;
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
  spoke_a_receiver: string;
  spoke_b_receiver: string;
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
  name: string;
  besuRpc: string;
  besuWs: string;
  htlcAddress: string;
  internalApiUrl: string;
  counterpartGrpc: string;
}

// ---------------------------------------------------------------------------
// HtlcRelay
// ---------------------------------------------------------------------------

/** Maximum number of events retained per category. Acts as a circular buffer. */
const MAX_EVENTS = 10_000;

export class HtlcRelay {
  private readonly settleEvents: SettleEvent[] = [];
  private readonly lockEvents: LockEvent[] = [];
  private readonly fxProposalEvents: FXProposalEvent[] = [];
  private readonly fxAcceptanceEvents: FXAcceptanceEvent[] = [];
  private readonly fxRejectionEvents: FXRejectionEvent[] = [];
  private readonly fxCancellationEvents: FXCancellationEvent[] = [];
  private readonly fxSettlementEvents: FXSettlementEvent[] = [];
  /**
   * Tracks secrets that this relay has already forwarded for settlement.
   * Prevents feedback loops: when the relay settles on Spoke-B, the resulting
   * on-chain LogHTLCClaimed event would be picked up again and erroneously
   * forwarded back to Spoke-A (where the HTLC is already settled).
   */
  private readonly forwardedSecrets = new Set<string>();
  /** Tracks trade IDs already forwarded cross-spoke. Value = timestamp (ms). */
  private readonly forwardedTradeIds = new Map<string, number>();

  constructor(
    private readonly spokes: SpokeDep[],
    private readonly protoPath: string,
    private readonly pollIntervalMs: number,
    private readonly relayAuthSecret: string,
    private readonly relayStore: RelayStore,
    private readonly cactiConnectors: Map<string, PluginLedgerConnectorBesu>,
    private readonly log: Pick<Console, "info" | "warn" | "error"> = console,
  ) {}

  // ── Public accessors (used by REST API) ────────────────────────────────

  getSettleEvents(sinceMs = 0): SettleEvent[] {
    return this.settleEvents.filter((e) => e.timestamp >= sinceMs);
  }

  getLockEvents(sinceMs = 0): LockEvent[] {
    return this.lockEvents.filter((e) => e.timestamp >= sinceMs);
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
   * Start polling all configured spokes. The provided AbortSignal is checked
   * at each poll tick; cancel it to shut the relay down gracefully.
   */
  start(signal: AbortSignal): void {
    for (const spoke of this.spokes) {
      this.pollSpoke(spoke, signal).catch((err: unknown) => {
        this.log.error(`[${spoke.name}] fatal poll error: ${String(err)}`);
      });
    }
  }

  // ── Private helpers ────────────────────────────────────────────────────

  private async pollSpoke(
    spoke: SpokeDep,
    signal: AbortSignal,
  ): Promise<void> {
    const connector = this.cactiConnectors.get(spoke.name);
    if (!connector) {
      this.log.error(`[${spoke.name}] no Cacti connector registered — cannot poll`);
      return;
    }

    // ethers Interface used only for ABI decoding of raw EvmLog topics/data.
    const iface = new ethers.Interface(HTLC_ABI);
    const grpcClient = createGrpcClient(spoke.counterpartGrpc, this.protoPath);

    // Compute event topic hashes for log filtering via Cacti getPastLogs.
    const topicLocked = ethers.id("LogHTLCLocked(bytes32,address,address,bytes32,uint256,bytes32)");
    const topicClaimed = ethers.id("LogHTLCClaimed(bytes32,bytes32)");

    // Determine the starting block via the Cacti connector's getBlock.
    let fromBlock: number;
    try {
      const blockResp = await connector.getBlock({ blockHashOrBlockNumber: "latest" });
      fromBlock = typeof blockResp.block === "object" && blockResp.block !== null
        ? Number((blockResp.block as Record<string, unknown>)["number"] ?? 0)
        : 0;
    } catch (err) {
      this.log.error(`[${spoke.name}] cannot get current block via Cacti: ${String(err)}`);
      fromBlock = 0;
    }
    this.log.info(`[${spoke.name}] relay started at block ${fromBlock} (via Cacti connector)`);

    while (!signal.aborted) {
      await sleep(this.pollIntervalMs);
      if (signal.aborted) break;

      try {
        // Get latest block number via Cacti connector.
        const latestResp = await connector.getBlock({ blockHashOrBlockNumber: "latest" });
        const toBlock = typeof latestResp.block === "object" && latestResp.block !== null
          ? Number((latestResp.block as Record<string, unknown>)["number"] ?? 0)
          : 0;
        if (toBlock < fromBlock) continue;

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
            const evt: LockEvent = {
              spoke: spoke.name,
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
            pushRing(this.lockEvents, evt, MAX_EVENTS);
            this.log.info(
              `[${spoke.name}] LogHTLCLocked contractId=${evt.contractId} block=${evt.blockNumber}`,
            );
          } catch (decodeErr) {
            this.log.warn(`[${spoke.name}] failed to decode LogHTLCLocked: ${String(decodeErr)}`);
          }
        }

        // Process LogHTLCClaimed events — decode and forward settlement.
        for (const raw of claimedResp.logs) {
          try {
            const parsed = iface.parseLog({ topics: raw.topics, data: raw.data });
            if (!parsed) continue;
            const contractId = strip0x(parsed.args[0] as string);
            const secret = strip0x(parsed.args[1] as string);
            const evt: SettleEvent = {
              spoke: spoke.name,
              contractId,
              secret,
              blockNumber: raw.blockNumber,
              txHash: raw.transactionHash,
              timestamp: Date.now(),
            };
            pushRing(this.settleEvents, evt, MAX_EVENTS);
            this.log.info(
              `[${spoke.name}] LogHTLCClaimed contractId=${contractId} block=${raw.blockNumber} tx=${raw.transactionHash}`,
            );

            if (this.forwardedSecrets.has(secret)) {
              this.log.info(
                `[${spoke.name}] skipping echo event for already-forwarded secret contractId=${contractId}`,
              );
              continue;
            }

            const counterpartId = this.resolveCounterpartContractId(
              spoke.name,
              contractId,
            );

            await this.settleOnCounterpart(
              grpcClient,
              spoke.name,
              counterpartId,
              secret,
            );
            this.forwardedSecrets.add(secret);
          } catch (decodeErr) {
            this.log.warn(`[${spoke.name}] failed to decode LogHTLCClaimed: ${String(decodeErr)}`);
          }
        }

        // ── FX Agreement polling (REST-based, no on-chain contract) ─────
        this.pruneForwardedTradeIds();
        await this.processDueRetriesForSpoke(grpcClient, spoke.name);
        await this.pollFXAgreementsRest(grpcClient, spoke);

        const stats = this.relayStore.getRetryStats();
        if (stats.pending > 0) {
          this.log.info(
            `[relay] retry-stats pending=${stats.pending} max_lag_ms=${stats.maxLagMs}`,
          );
        }

        fromBlock = toBlock + 1;
      } catch (err) {
        this.log.warn(`[${spoke.name}] poll cycle error: ${String(err)}`);
      }
    }

    grpcClient.close();
    this.log.info(`[${spoke.name}] relay stopped`);
  }

  /**
   * Given a source spoke name and contractId, find the corresponding lock on
   * the counterpart spoke by matching the hashLock field.
   * Falls back to the source contractId (with a warning) when no matching
   * counterpart lock is in the ring buffer yet.
   */
  private resolveCounterpartContractId(
    spokeName: string,
    contractId: string,
  ): string {
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
        `[${spokeName}] resolveCounterpart: source lock not found for contractId=${contractId}, forwarding as-is`,
      );
      return contractId;
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
        `[${spokeName}] resolveCounterpart: no counterpart lock found for hashLock=${sourceLock.hashLock}, forwarding source contractId`,
      );
      return contractId;
    }

    this.log.info(
      `[${spokeName}] resolveCounterpart: ${contractId} → ${counterpartLock.contractId} (spoke=${counterpartLock.spoke})`,
    );
    return counterpartLock.contractId;
  }

  private settleOnCounterpart(
    client: PaymentOrchestratorClient,
    spokeName: string,
    contractId: string,
    secret: string,
  ): Promise<void> {
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
          } else {
            this.log.info(
              `[${spokeName}] counterpart settled contractId=${contractId} htlcTx=${resp?.htlc_tx_hash} zetoTx=${resp?.zeto_tx_hash}`,
            );
          }
          resolve();
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
    client: PaymentOrchestratorClient,
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
        this.log.warn(`[${spoke.name}] FX REST poll failed: ${res.status}`);
        return;
      }
      body = await res.json() as { agreements?: unknown[] };
    } catch (err) {
      this.log.warn(`[${spoke.name}] FX REST poll error: ${String(err)}`);
      return;
    }

    const agreements = body.agreements ?? [];
    for (const raw of agreements) {
      const a = raw as Record<string, unknown>;
      const tradeId = a["trade_id"] as string;
      const state = a["state"] as string;
      if (!tradeId || !state) continue;

      if (state === "FX_STATE_PROPOSED" && !(await this.wasForwarded(`propose:${tradeId}`))) {
        const evt: FXProposalEvent = {
          spoke: spoke.name,
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
          spokeAReceiver: (a["spoke_a_receiver"] as string) ?? "",
          spokenBReceiver: (a["spoke_b_receiver"] as string) ?? "",
          blockNumber: 0,
          txHash: "",
          timestamp: Date.now(),
        };
        pushRing(this.fxProposalEvents, evt, MAX_EVENTS);
        this.log.info(`[${spoke.name}] FX REST: forwarding proposal tradeId=${tradeId}`);
        await this.tryForwardFXAction("propose", spoke.name, tradeId, client, evt as unknown as Record<string, unknown>);

      } else if (state === "FX_STATE_ACCEPTED" && !(await this.wasForwarded(`accept:${tradeId}`))) {
        const evt: FXAcceptanceEvent = {
          spoke: spoke.name,
          tradeId,
          blockNumber: 0,
          txHash: "",
          timestamp: Date.now(),
        };
        pushRing(this.fxAcceptanceEvents, evt, MAX_EVENTS);
        this.log.info(`[${spoke.name}] FX REST: forwarding acceptance tradeId=${tradeId}`);
        await this.tryForwardFXAction("accept", spoke.name, tradeId, client);

      } else if (state === "FX_STATE_REJECTED" && !(await this.wasForwarded(`reject:${tradeId}`))) {
        const evt: FXRejectionEvent = {
          spoke: spoke.name,
          tradeId,
          blockNumber: 0,
          txHash: "",
          timestamp: Date.now(),
        };
        pushRing(this.fxRejectionEvents, evt, MAX_EVENTS);
        this.log.info(`[${spoke.name}] FX REST: forwarding rejection tradeId=${tradeId}`);
        await this.tryForwardFXAction("reject", spoke.name, tradeId, client);

      } else if (state === "FX_STATE_CANCELLED" && !(await this.wasForwarded(`cancel:${tradeId}`))) {
        const evt: FXCancellationEvent = {
          spoke: spoke.name,
          tradeId,
          blockNumber: 0,
          txHash: "",
          timestamp: Date.now(),
        };
        pushRing(this.fxCancellationEvents, evt, MAX_EVENTS);
        this.log.info(`[${spoke.name}] FX REST: forwarding cancellation tradeId=${tradeId}`);
        await this.tryForwardFXAction("cancel", spoke.name, tradeId, client);

      } else if (state === "FX_STATE_SETTLED" && !(await this.wasForwarded(`settle:${tradeId}`))) {
        const evt: FXSettlementEvent = {
          spoke: spoke.name,
          tradeId,
          blockNumber: 0,
          txHash: "",
          timestamp: Date.now(),
        };
        pushRing(this.fxSettlementEvents, evt, MAX_EVENTS);
        this.log.info(`[${spoke.name}] FX REST: forwarding settlement tradeId=${tradeId}`);
        await this.tryForwardFXAction("settle", spoke.name, tradeId, client);
      }
    }
  }

  private async processDueRetriesForSpoke(
    client: PaymentOrchestratorClient,
    spokeName: string,
  ): Promise<void> {
    const due = this.relayStore.getDueRetries(spokeName);
    for (const item of due) {
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
          spoke_a_receiver: event.spokeAReceiver,
          spoke_b_receiver: event.spokenBReceiver,
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

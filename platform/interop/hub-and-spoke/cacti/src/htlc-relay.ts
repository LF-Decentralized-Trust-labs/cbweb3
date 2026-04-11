/**
 * HtlcRelay — core interoperability logic for the Cacti HTLC cross-spoke relay.
 *
 * Responsibilities:
 *   1. Poll each Besu spoke for LogHTLCClaimed and LogHTLCLocked events.
 *   2. On LogHTLCClaimed: call SettleHTLC on the counterpart spoke's
 *      payment-orchestrator via gRPC, propagating the revealed secret.
 *   3. Store all observed events in an in-memory ring buffer so the REST API
 *      (consumed by the CactiRelay Go adapter) can poll them by timestamp.
 *
 * This module uses ethers.js JsonRpcProvider for HTTP-based log polling
 * (matching the approach of the previous Go relay) and @grpc/grpc-js for
 * the payment-orchestrator client.
 */

import { ethers } from "ethers";
import * as grpc from "@grpc/grpc-js";
import * as protoLoader from "@grpc/proto-loader";

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
    const provider = new ethers.JsonRpcProvider(spoke.besuRpc);
    const contract = new ethers.Contract(spoke.htlcAddress, HTLC_ABI, provider);
    const grpcClient = createGrpcClient(spoke.counterpartGrpc, this.protoPath);

    let fromBlock: number;
    try {
      fromBlock = await provider.getBlockNumber();
    } catch (err) {
      this.log.error(
        `[${spoke.name}] cannot get current block: ${String(err)}`,
      );
      fromBlock = 0;
    }
    this.log.info(`[${spoke.name}] relay started at block ${fromBlock}`);

    while (!signal.aborted) {
      await sleep(this.pollIntervalMs);
      if (signal.aborted) break;

      try {
        const toBlock = await provider.getBlockNumber();
        if (toBlock < fromBlock) continue;

        const [claimedLogs, lockedLogs] = await Promise.all([
          contract.queryFilter(contract.filters["LogHTLCClaimed"](), fromBlock, toBlock),
          contract.queryFilter(contract.filters["LogHTLCLocked"](), fromBlock, toBlock),
        ]);

        for (const log of lockedLogs) {
          const e = log as ethers.EventLog;
          const evt: LockEvent = {
            spoke: spoke.name,
            contractId: strip0x(e.args[0] as string),
            sender: e.args[1] as string,
            receiver: e.args[2] as string,
            hashLock: strip0x(e.args[3] as string),
            timeLock: Number(e.args[4]),
            zetoLockRef: strip0x(e.args[5] as string),
            blockNumber: e.blockNumber,
            txHash: e.transactionHash,
            timestamp: Date.now(),
          };
          pushRing(this.lockEvents, evt, MAX_EVENTS);
          this.log.info(
            `[${spoke.name}] LogHTLCLocked contractId=${evt.contractId} block=${evt.blockNumber}`,
          );
        }

        for (const log of claimedLogs) {
          const e = log as ethers.EventLog;
          const contractId = strip0x(e.args[0] as string);
          const secret = strip0x(e.args[1] as string);
          const evt: SettleEvent = {
            spoke: spoke.name,
            contractId,
            secret,
            blockNumber: e.blockNumber,
            txHash: e.transactionHash,
            timestamp: Date.now(),
          };
          pushRing(this.settleEvents, evt, MAX_EVENTS);
          this.log.info(
            `[${spoke.name}] LogHTLCClaimed contractId=${contractId} block=${e.blockNumber} tx=${e.transactionHash}`,
          );

          // Dedup: skip echo events caused by a previous relay-initiated settle.
          // Without this, the relay would enter a feedback loop:
          //   Spoke-A settle → relay forwards to Spoke-B → Spoke-B emits event
          //   → relay tries to forward back to Spoke-A → NOT_FOUND error.
          if (this.forwardedSecrets.has(secret)) {
            this.log.info(
              `[${spoke.name}] skipping echo event for already-forwarded secret contractId=${contractId}`,
            );
            continue;
          }

          // Resolve the counterpart spoke's contractId via hashLock.
          // Each spoke has its own contractId for the same HTLC; the shared
          // link between them is the hashLock. Sending the source contractId
          // to the counterpart payment orchestrator yields NOT_FOUND.
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
        }

        // ── FX Agreement polling (REST-based, no on-chain contract) ─────
        this.pruneForwardedTradeIds();
        await this.pollFXAgreementsRest(grpcClient, spoke);

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
      const res = await fetch(url);
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

      if (state === "FX_STATE_PROPOSED" && !this.forwardedTradeIds.has(`propose:${tradeId}`)) {
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
        await this.proposeOnCounterpart(client, spoke.name, evt);
        this.forwardedTradeIds.set(`propose:${tradeId}`, Date.now());

      } else if (state === "FX_STATE_ACCEPTED" && !this.forwardedTradeIds.has(`accept:${tradeId}`)) {
        const evt: FXAcceptanceEvent = {
          spoke: spoke.name,
          tradeId,
          blockNumber: 0,
          txHash: "",
          timestamp: Date.now(),
        };
        pushRing(this.fxAcceptanceEvents, evt, MAX_EVENTS);
        this.log.info(`[${spoke.name}] FX REST: forwarding acceptance tradeId=${tradeId}`);
        await this.acceptOnCounterpart(client, spoke.name, tradeId);
        this.forwardedTradeIds.set(`accept:${tradeId}`, Date.now());

      } else if (state === "FX_STATE_REJECTED" && !this.forwardedTradeIds.has(`reject:${tradeId}`)) {
        const evt: FXRejectionEvent = {
          spoke: spoke.name,
          tradeId,
          blockNumber: 0,
          txHash: "",
          timestamp: Date.now(),
        };
        pushRing(this.fxRejectionEvents, evt, MAX_EVENTS);
        this.log.info(`[${spoke.name}] FX REST: forwarding rejection tradeId=${tradeId}`);
        await this.rejectOnCounterpart(client, spoke.name, tradeId);
        this.forwardedTradeIds.set(`reject:${tradeId}`, Date.now());
      }
    }
  }

  private proposeOnCounterpart(
    client: PaymentOrchestratorClient,
    spokeName: string,
    event: FXProposalEvent,
  ): Promise<void> {
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
            this.log.error(
              `[${spokeName}] ProposeFXAgreement gRPC failed tradeId=${event.tradeId}: ${err.message}`,
            );
          } else {
            this.log.info(
              `[${spokeName}] counterpart proposed tradeId=${event.tradeId} tx=${resp?.tx_hash}`,
            );
          }
          resolve();
        },
      );
    });
  }

  private acceptOnCounterpart(
    client: PaymentOrchestratorClient,
    spokeName: string,
    tradeId: string,
  ): Promise<void> {
    return new Promise((resolve) => {
      const deadline = new Date(Date.now() + 30_000);
      client.AcceptFXAgreement(
        { trade_id: tradeId, on_behalf: true },
        new grpc.Metadata(),
        { deadline },
        (err: grpc.ServiceError | null, resp: AcceptFXAgreementGrpcResponse) => {
          if (err) {
            this.log.error(
              `[${spokeName}] AcceptFXAgreement gRPC failed tradeId=${tradeId}: ${err.message}`,
            );
          } else {
            this.log.info(
              `[${spokeName}] counterpart accepted tradeId=${tradeId} tx=${resp?.tx_hash}`,
            );
          }
          resolve();
        },
      );
    });
  }

  private rejectOnCounterpart(
    client: PaymentOrchestratorClient,
    spokeName: string,
    tradeId: string,
  ): Promise<void> {
    return new Promise((resolve) => {
      const deadline = new Date(Date.now() + 30_000);
      client.RejectFXAgreement(
        { trade_id: tradeId, on_behalf: true },
        new grpc.Metadata(),
        { deadline },
        (err: grpc.ServiceError | null, resp: RejectFXAgreementGrpcResponse) => {
          if (err) {
            this.log.error(
              `[${spokeName}] RejectFXAgreement gRPC failed tradeId=${tradeId}: ${err.message}`,
            );
          } else {
            this.log.info(
              `[${spokeName}] counterpart rejected tradeId=${tradeId} tx=${resp?.tx_hash}`,
            );
          }
          resolve();
        },
      );
    });
  }
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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

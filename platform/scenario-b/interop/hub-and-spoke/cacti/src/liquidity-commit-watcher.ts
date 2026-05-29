/**
 * LiquidityCommitWatcher — Cacti module for 007-bridge-based-cb-liquidity.
 *
 * Watches the `CommitMatched` event from LiquidityCommitRegistry on the Hub Besu network.
 * When the event fires, POSTs the decoded payload to each configured gateway's
 * `/internal/amm/execute-matched-commit` endpoint so the respective sovereign CB
 * can add its single-sided liquidity autonomously.
 *
 * Environment variables consumed:
 *   LIQUIDITY_COMMIT_REGISTRY_ADDRESS  — LCR contract address on Hub (0x-prefixed)
 *   HUB_BESU_RPC                       — Hub Besu HTTP RPC URL
 *   HUB_BESU_WS                        — Hub Besu WebSocket URL
 *   GATEWAY_INTERNAL_URLS              — Comma-separated list of gateway internal URLs
 *                                        (e.g. "http://gateway-a:18080,http://gateway-b:18081")
 *   INTERNAL_RELAY_AUTH_SECRET         — Shared secret for X-Relay-Auth header
 *   POLL_INTERVAL_MS                   — Event poll interval (default: 5000)
 *   LCR_WATCHER_START_BLOCK            — Block to start watching from (default: 0)
 */

import { ethers } from "ethers";
import type { PluginLedgerConnectorBesu } from "@hyperledger/cactus-plugin-ledger-connector-besu";

// ---------------------------------------------------------------------------
// ABI — only the CommitMatched event is needed for watching.
// Keep in sync with contracts/src/interfaces/ILiquidityCommitRegistry.sol.
// ---------------------------------------------------------------------------

const COMMIT_MATCHED_ABI = [
  "event CommitMatched(string indexed poolPair, bytes32 commitIdA, address signerA, uint256 amountA, bytes32 commitIdB, address signerB, uint256 amountB)",
];

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface CommitMatchedEvent {
  poolPair:  string;  // decoded via topic — Note: indexed string is hashed on-chain
  commitIdA: string;  // bytes32 hex
  signerA:   string;  // address hex
  amountA:   string;  // decimal string (wei)
  commitIdB: string;  // bytes32 hex
  signerB:   string;  // address hex
  amountB:   string;  // decimal string (wei)
  blockNumber: number;
  txHash:    string;
  timestamp: number;  // Unix ms when this process observed the event
}

interface ExecuteMatchedCommitPayload {
  pool_pair:    string;
  commit_id_a:  string;
  signer_a:     string;
  amount_a:     string;
  commit_id_b:  string;
  signer_b:     string;
  amount_b:     string;
}

// ---------------------------------------------------------------------------
// LiquidityCommitWatcher
// ---------------------------------------------------------------------------

export class LiquidityCommitWatcher {
  private readonly iface: ethers.Interface;
  private readonly contractAddress: string;
  private readonly gatewayUrls: string[];
  private readonly relayAuthSecret: string;
  private readonly pollIntervalMs: number;
  private readonly startBlock: number;
  private lastProcessedBlock: number;
  private running = false;
  private abortSignal?: AbortSignal;

  // Optional Cacti connector for Besu integration.
  private readonly connector?: PluginLedgerConnectorBesu;
  // Fallback ethers provider for direct polling when no Cacti connector is provided.
  private provider?: ethers.JsonRpcProvider;

  constructor(opts: {
    contractAddress: string;
    gatewayUrls: string[];
    relayAuthSecret: string;
    pollIntervalMs?: number;
    startBlock?: number;
    connector?: PluginLedgerConnectorBesu;
    hubRpc?: string;
  }) {
    this.iface           = new ethers.Interface(COMMIT_MATCHED_ABI);
    this.contractAddress = opts.contractAddress.toLowerCase();
    this.gatewayUrls     = opts.gatewayUrls.filter(u => u.trim() !== "");
    this.relayAuthSecret = opts.relayAuthSecret;
    this.pollIntervalMs  = opts.pollIntervalMs ?? 5_000;
    this.startBlock      = opts.startBlock ?? 0;
    this.lastProcessedBlock = this.startBlock;
    this.connector       = opts.connector;
    if (!opts.connector && opts.hubRpc) {
      this.provider = new ethers.JsonRpcProvider(opts.hubRpc);
    }
  }

  /**
   * Start watching for CommitMatched events.
   * Polls getPastLogs via Cacti connector (preferred) or ethers provider (fallback).
   */
  start(signal?: AbortSignal): void {
    if (this.running) return;
    this.running = true;
    this.abortSignal = signal;
    console.log(
      `[LiquidityCommitWatcher] starting — contract=${this.contractAddress} ` +
      `gateways=${this.gatewayUrls.join(",")} pollMs=${this.pollIntervalMs}`,
    );
    this.pollLoop().catch((err: unknown) => {
      console.error("[LiquidityCommitWatcher] fatal poll loop error:", err);
    });
  }

  private async pollLoop(): Promise<void> {
    const commitMatchedTopic = this.iface.getEvent("CommitMatched")!.topicHash;

    while (this.running && !this.abortSignal?.aborted) {
      try {
        const toBlock = await this.getLatestBlock();
        const fromBlock = this.lastProcessedBlock + 1;

        if (fromBlock <= toBlock) {
          const logs = await this.getLogs(fromBlock, toBlock, commitMatchedTopic);
          for (const log of logs) {
            try {
              const ev = this.decodeLog(log);
              if (ev) {
                await this.forwardToGateways(ev);
              }
            } catch (decodeErr) {
              console.warn("[LiquidityCommitWatcher] decode error:", decodeErr);
            }
          }
          this.lastProcessedBlock = toBlock;
        }
      } catch (err) {
        console.error("[LiquidityCommitWatcher] poll error:", err);
      }

      await this.sleep(this.pollIntervalMs);
    }

    console.log("[LiquidityCommitWatcher] stopped.");
  }

  private async getLatestBlock(): Promise<number> {
    if (this.connector) {
      const resp = await this.connector.getBlock({ blockHashOrBlockNumber: "latest" });
      return Number(resp.block.number ?? 0);
    }
    if (this.provider) {
      return await this.provider.getBlockNumber();
    }
    throw new Error("LiquidityCommitWatcher: no connector or provider configured");
  }

  private async getLogs(
    fromBlock: number,
    toBlock: number,
    topicHash: string,
  ): Promise<EthLog[]> {
    if (this.connector) {
      const resp = await this.connector.getPastLogs({
        fromBlock: "0x" + fromBlock.toString(16),
        toBlock:   "0x" + toBlock.toString(16),
        address:   this.contractAddress,
        topics:    [topicHash],
      });
      return (resp.logs ?? []) as EthLog[];
    }
    if (this.provider) {
      const logs = await this.provider.getLogs({
        fromBlock,
        toBlock,
        address: this.contractAddress,
        topics:  [topicHash],
      });
      return logs.map(l => ({
        topics: l.topics as string[],
        data:   l.data,
        blockNumber: typeof l.blockNumber === "number" ? l.blockNumber : Number(l.blockNumber),
        transactionHash: l.transactionHash,
      }));
    }
    return [];
  }

  private decodeLog(log: EthLog): CommitMatchedEvent | null {
    try {
      // CommitMatched has `poolPair` as indexed (topic[1] = keccak256 hash — cannot recover string).
      // commitIdA, signerA, amountA, commitIdB, signerB, amountB are non-indexed (in data).
      const nonIndexedResult = this.iface.decodeEventLog(
        "CommitMatched",
        log.data,
        log.topics,
      );

      // poolPair is indexed and therefore hashed — we can only read its keccak256 in topics[1].
      // The sovereign service will match by signerA/signerB instead of poolPair string.
      // We store the topic hash as the poolPair value and let each gateway correlate.
      const poolPairHash = log.topics[1] ?? "0x";

      // ethers decodeEventLog (with topics) returns ALL parameters in declaration order,
      // including the indexed poolPair at index 0 (as its keccak256 hash).
      // Non-indexed params therefore start at index 1:
      //   [0] poolPair (keccak256 hash, indexed)
      //   [1] commitIdA   (bytes32)
      //   [2] signerA     (address)
      //   [3] amountA     (uint256 → BigInt)
      //   [4] commitIdB   (bytes32)
      //   [5] signerB     (address)
      //   [6] amountB     (uint256 → BigInt)
      return {
        poolPair:  poolPairHash, // hashed — see note above
        commitIdA: nonIndexedResult[1] as string,
        signerA:   nonIndexedResult[2] as string,
        amountA:   (nonIndexedResult[3] as bigint).toString(),
        commitIdB: nonIndexedResult[4] as string,
        signerB:   nonIndexedResult[5] as string,
        amountB:   (nonIndexedResult[6] as bigint).toString(),
        blockNumber: log.blockNumber,
        txHash:    log.transactionHash,
        timestamp: Date.now(),
      };
    } catch {
      return null;
    }
  }

  private async forwardToGateways(ev: CommitMatchedEvent): Promise<void> {
    const payload: ExecuteMatchedCommitPayload = {
      pool_pair:   ev.poolPair,
      commit_id_a: ev.commitIdA,
      signer_a:    ev.signerA,
      amount_a:    ev.amountA,
      commit_id_b: ev.commitIdB,
      signer_b:    ev.signerB,
      amount_b:    ev.amountB,
    };

    console.log(
      `[LiquidityCommitWatcher] CommitMatched block=${ev.blockNumber} tx=${ev.txHash} ` +
      `signerA=${ev.signerA} signerB=${ev.signerB} — forwarding to ${this.gatewayUrls.length} gateway(s)`,
    );

    const results = await Promise.allSettled(
      this.gatewayUrls.map(url => this.postToGateway(url, payload)),
    );

    for (let i = 0; i < results.length; i++) {
      const r = results[i];
      if (r.status === "rejected") {
        console.warn(
          `[LiquidityCommitWatcher] gateway ${this.gatewayUrls[i]} failed: ${String(r.reason)}`,
        );
      }
    }
  }

  private async postToGateway(baseUrl: string, payload: ExecuteMatchedCommitPayload): Promise<void> {
    const url = `${baseUrl.replace(/\/$/, "")}/internal/amm/execute-matched-commit`;
    const resp = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type":  "application/json",
        "X-Relay-Auth":  this.relayAuthSecret,
      },
      body: JSON.stringify(payload),
    });
    if (!resp.ok) {
      const body = await resp.text().catch(() => "");
      throw new Error(`HTTP ${resp.status} from ${url}: ${body}`);
    }
    const json = await resp.json().catch(() => ({})) as Record<string, unknown>;
    if (json["status"] === "ignored") {
      // The gateway determined this commit does not belong to it — expected.
      return;
    }
    console.log(`[LiquidityCommitWatcher] gateway ${baseUrl} → status=${json["status"] ?? "ok"}`);
  }

  private sleep(ms: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, ms));
  }

  stop(): void {
    this.running = false;
  }
}

// ---------------------------------------------------------------------------
// Internal log type (compatible with both Cacti getPastLogs and ethers getLogs)
// ---------------------------------------------------------------------------

interface EthLog {
  topics: string[];
  data: string;
  blockNumber: number;
  transactionHash: string;
}

// ---------------------------------------------------------------------------
// Factory — constructs watcher from environment variables.
// ---------------------------------------------------------------------------

export function createLiquidityCommitWatcherFromEnv(
  connector?: PluginLedgerConnectorBesu,
): LiquidityCommitWatcher | null {
  const contractAddress = process.env["LIQUIDITY_COMMIT_REGISTRY_ADDRESS"] ?? "";
  const gatewayUrls     = (process.env["GATEWAY_INTERNAL_URLS"] ?? "")
    .split(",")
    .map(u => u.trim())
    .filter(u => u !== "");
  const relayAuthSecret = process.env["INTERNAL_RELAY_AUTH_SECRET"] ?? "";
  const hubRpc          = process.env["HUB_BESU_RPC"] ?? "";
  const pollIntervalMs  = parseInt(process.env["POLL_INTERVAL_MS"] ?? "5000", 10);
  const startBlock      = parseInt(process.env["LCR_WATCHER_START_BLOCK"] ?? "0", 10);

  if (!contractAddress) {
    console.log(
      "[LiquidityCommitWatcher] LIQUIDITY_COMMIT_REGISTRY_ADDRESS not set — watcher disabled",
    );
    return null;
  }
  if (gatewayUrls.length === 0) {
    console.log(
      "[LiquidityCommitWatcher] GATEWAY_INTERNAL_URLS not set — watcher disabled",
    );
    return null;
  }
  if (!relayAuthSecret) {
    console.error(
      "[LiquidityCommitWatcher] INTERNAL_RELAY_AUTH_SECRET not set — refusing to start (security)",
    );
    return null;
  }

  return new LiquidityCommitWatcher({
    contractAddress,
    gatewayUrls,
    relayAuthSecret,
    pollIntervalMs,
    startBlock: isNaN(startBlock) ? 0 : startBlock,
    connector,
    hubRpc,
  });
}

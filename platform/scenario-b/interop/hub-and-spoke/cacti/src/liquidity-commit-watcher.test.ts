// SPDX-License-Identifier: Apache-2.0

import { test } from "node:test";
import assert from "node:assert/strict";
import * as http from "node:http";
import { promises as fs } from "fs";
import * as os from "os";
import * as path from "path";
import { ethers } from "ethers";
import type { PluginLedgerConnectorBesu } from "@hyperledger/cactus-plugin-ledger-connector-besu";

import { BlockWatermarkStore } from "./block-watermark-store";
import { LiquidityCommitWatcher } from "./liquidity-commit-watcher";

const CONTRACT = "0x00000000000000000000000000000000000000aa";
const GENESIS = "0x" + "11".repeat(32);

const EVENT_ABI = [
  "event CommitMatched(string indexed poolPair, bytes32 commitIdA, address signerA, uint256 amountA, bytes32 commitIdB, address signerB, uint256 amountB)",
];
const iface = new ethers.Interface(EVENT_ABI);
const COMMIT_TOPIC = iface.getEvent("CommitMatched")!.topicHash;

interface FakeLog {
  topics: string[];
  data: string;
  blockNumber: number;
  transactionHash: string;
  logIndex: number;
}

// Build a decodable CommitMatched log at a given block/tx/logIndex.
function makeLog(blockNumber: number, txHash: string, logIndex = 0): FakeLog {
  const data = ethers.AbiCoder.defaultAbiCoder().encode(
    ["bytes32", "address", "uint256", "bytes32", "address", "uint256"],
    [
      "0x" + "a1".repeat(32),
      "0x00000000000000000000000000000000000000A1",
      1_000n,
      "0x" + "b2".repeat(32),
      "0x00000000000000000000000000000000000000B2",
      2_000n,
    ],
  );
  return {
    topics: [COMMIT_TOPIC, ethers.id("BRL/COP")],
    data,
    blockNumber,
    transactionHash: txHash,
    logIndex,
  };
}

// Minimal fake Cacti connector: fixed latest block + genesis hash, returns configured logs
// filtered to the requested [fromBlock, toBlock] range. Records every getPastLogs fromBlock.
function fakeConnector(opts: {
  latestBlock: number;
  logs?: FakeLog[];
  genesisHash?: string;
  capture?: { fromBlocks: string[] };
}): PluginLedgerConnectorBesu {
  const logs = opts.logs ?? [];
  return {
    async getBlock(req: { blockHashOrBlockNumber: string | number }) {
      if (req.blockHashOrBlockNumber === 0) {
        return { block: { number: 0, hash: opts.genesisHash ?? GENESIS } };
      }
      return { block: { number: opts.latestBlock } };
    },
    async getPastLogs(args: { fromBlock: string; toBlock: string }) {
      opts.capture?.fromBlocks.push(args.fromBlock);
      const from = parseInt(args.fromBlock, 16);
      const to = parseInt(args.toBlock, 16);
      return { logs: logs.filter(l => l.blockNumber >= from && l.blockNumber <= to) };
    },
  } as unknown as PluginLedgerConnectorBesu;
}

// A local gateway that records POSTs and can be told to fail.
function gatewayServer(): Promise<{
  url: string;
  hits: () => number;
  setFail: (fail: boolean) => void;
  close: () => Promise<void>;
}> {
  return new Promise(resolve => {
    let count = 0;
    let fail = false;
    const server = http.createServer((req, res) => {
      count++;
      if (fail) {
        res.writeHead(500).end("boom");
        return;
      }
      res.writeHead(200, { "Content-Type": "application/json" }).end(JSON.stringify({ status: "ok" }));
    });
    server.listen(0, "127.0.0.1", () => {
      const addr = server.address() as { port: number };
      resolve({
        url: `http://127.0.0.1:${addr.port}`,
        hits: () => count,
        setFail: (f: boolean) => { fail = f; },
        close: () => new Promise<void>(r => server.close(() => r())),
      });
    });
  });
}

async function tmpFile(): Promise<string> {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), "lcr-watcher-test-"));
  return path.join(dir, "store.json");
}

function sleep(ms: number): Promise<void> {
  return new Promise(r => setTimeout(r, ms));
}

// Poll `cond` until true or timeout — avoids fixed-duration flakiness.
async function waitFor(cond: () => boolean | Promise<boolean>, timeoutMs = 2_000): Promise<boolean> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (await cond()) return true;
    await sleep(10);
  }
  return await cond();
}

function newWatcher(file: string, gatewayUrl: string, connector: PluginLedgerConnectorBesu, startBlock = 0) {
  return new LiquidityCommitWatcher({
    contractAddress: CONTRACT,
    gatewayUrls: [gatewayUrl],
    relayAuthSecret: "secret",
    pollIntervalMs: 10,
    startBlock,
    connector,
    watermarkStore: new BlockWatermarkStore(file),
  });
}

test("resumes from the persisted block instead of startBlock", async () => {
  const file = await tmpFile();
  const seed = new BlockWatermarkStore(file);
  await seed.init();
  await seed.set(CONTRACT, 500);

  const capture = { fromBlocks: [] as string[] };
  const gw = await gatewayServer();
  const watcher = newWatcher(file, gw.url, fakeConnector({ latestBlock: 600, capture }));
  const controller = new AbortController();

  watcher.start(controller.signal);
  await waitFor(() => capture.fromBlocks.length > 0);
  controller.abort();
  watcher.stop();
  await gw.close();

  // First scan starts at persisted+1 (501 = 0x1f5), not startBlock+1.
  assert.equal(capture.fromBlocks[0], "0x" + (501).toString(16));

  const reloaded = new BlockWatermarkStore(file);
  await reloaded.init();
  assert.equal(reloaded.get(CONTRACT), 600);
});

test("does not re-forward an already-delivered event after restart", async () => {
  const file = await tmpFile();
  const tx = "0x" + "cc".repeat(32);
  const log = makeLog(510, tx, 0);

  // Simulate a crash AFTER forwarding but BEFORE the watermark advanced: the delivered key is
  // persisted, the watermark is still behind the event's block.
  const seed = new BlockWatermarkStore(file);
  await seed.init();
  await seed.set(CONTRACT, 509);
  await seed.markDelivered(`${tx}:0`);

  const gw = await gatewayServer();
  const watcher = newWatcher(file, gw.url, fakeConnector({ latestBlock: 510, logs: [log] }));
  const controller = new AbortController();

  watcher.start(controller.signal);
  // Watermark must reach 510 (block processed) ...
  const reloaded = new BlockWatermarkStore(file);
  await waitFor(async () => { await reloaded.init(); return reloaded.get(CONTRACT) === 510; });
  await sleep(30);
  controller.abort();
  watcher.stop();
  await gw.close();

  // ... but the gateway must NOT have been called: the event was already delivered.
  assert.equal(gw.hits(), 0, "already-delivered event must not be re-forwarded");
});

test("holds the watermark and retries when a gateway delivery fails", async () => {
  const file = await tmpFile();
  const tx = "0x" + "dd".repeat(32);
  const log = makeLog(511, tx, 0);

  const seed = new BlockWatermarkStore(file);
  await seed.init();
  await seed.set(CONTRACT, 510);

  const gw = await gatewayServer();
  gw.setFail(true); // gateway is down / erroring
  const watcher = newWatcher(file, gw.url, fakeConnector({ latestBlock: 511, logs: [log] }));
  const controller = new AbortController();

  watcher.start(controller.signal);
  // Give the loop several cycles while the gateway fails.
  await waitFor(() => gw.hits() >= 2);

  // Watermark must NOT advance past the failed block, and the event must NOT be marked delivered.
  const mid = new BlockWatermarkStore(file);
  await mid.init();
  assert.equal(mid.get(CONTRACT), 510, "watermark must not advance past a failed delivery");
  assert.equal(mid.hasDelivered(`${tx}:0`), false, "failed delivery must not be marked delivered");

  // Recover: gateway comes back → event delivered and watermark advances exactly once.
  gw.setFail(false);
  const done = new BlockWatermarkStore(file);
  await waitFor(async () => { await done.init(); return done.get(CONTRACT) === 511; });
  controller.abort();
  watcher.stop();
  await gw.close();

  assert.equal(done.get(CONTRACT), 511);
  assert.equal(done.hasDelivered(`${tx}:0`), true);
});

test("warns and forwards nothing when chain head is behind the watermark", async () => {
  const file = await tmpFile();
  const seed = new BlockWatermarkStore(file);
  await seed.init();
  await seed.set(CONTRACT, 900);
  await seed.setMeta(CONTRACT, GENESIS); // same chain — not a reset

  const gw = await gatewayServer();
  // Head (100) is far below the persisted watermark (900).
  const watcher = newWatcher(file, gw.url, fakeConnector({ latestBlock: 100, logs: [makeLog(50, "0x" + "ee".repeat(32))] }));
  const controller = new AbortController();

  watcher.start(controller.signal);
  await sleep(80);
  controller.abort();
  watcher.stop();
  await gw.close();

  assert.equal(gw.hits(), 0, "must not forward when head is below watermark");
  const reloaded = new BlockWatermarkStore(file);
  await reloaded.init();
  assert.equal(reloaded.get(CONTRACT), 900, "watermark unchanged while head is below it");
});

test("resets the watermark when the chain genesis changes (chain reset)", async () => {
  const file = await tmpFile();
  const seed = new BlockWatermarkStore(file);
  await seed.init();
  await seed.set(CONTRACT, 900);
  await seed.setMeta(CONTRACT, "0x" + "99".repeat(32)); // old chain identity

  const gw = await gatewayServer();
  // New chain: different genesis hash, head at 5, one event at block 3.
  const newGenesis = "0x" + "22".repeat(32);
  const tx = "0x" + "ff".repeat(32);
  const connector = fakeConnector({ latestBlock: 5, genesisHash: newGenesis, logs: [makeLog(3, tx)] });
  const watcher = newWatcher(file, gw.url, connector, /* startBlock */ 0);
  const controller = new AbortController();

  watcher.start(controller.signal);
  // After reset to startBlock 0, the block-3 event is scanned and forwarded, watermark → 5.
  const reloaded = new BlockWatermarkStore(file);
  await waitFor(async () => { await reloaded.init(); return reloaded.get(CONTRACT) === 5; });
  await sleep(20);
  controller.abort();
  watcher.stop();
  await gw.close();

  assert.equal(reloaded.get(CONTRACT), 5, "watermark re-synced from new genesis");
  assert.equal(reloaded.getMeta(CONTRACT), newGenesis, "new chain identity recorded");
  assert.ok(gw.hits() >= 1, "event on the reset chain must be delivered");
});

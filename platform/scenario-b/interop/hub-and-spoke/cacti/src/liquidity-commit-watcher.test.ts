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

// ── adaptive scan window ──────────────────────────────────────────────────────────────────────
//
// A spoke can fall far behind the head after the hub Besu restarts under a longer-lived relay.
// Each cycle then issued a getPastLogs over a fixed 5,000-block window; a node that cannot serve
// a range that wide drops the socket, so the same request failed again on every cycle and the
// watermark never moved. A request too big to succeed must not be repeated unchanged for ever.

// A connector whose getPastLogs can reject wide ranges (a node's log-range limit) or fail
// outright (an unreachable node), and whose head can move. Every requested width is recorded.
function windowConnector(opts: {
  head: () => number;
  maxRange?: number;
  failLogs?: () => boolean;
  onReject?: () => void; // called when a range is rejected for being too wide
  widths: number[];
}): PluginLedgerConnectorBesu {
  return {
    async getBlock(req: { blockHashOrBlockNumber: string | number }) {
      if (req.blockHashOrBlockNumber === 0) return { block: { number: 0, hash: GENESIS } };
      return { block: { number: opts.head() } };
    },
    async getPastLogs(args: { fromBlock: string; toBlock: string }) {
      const width = parseInt(args.toBlock, 16) - parseInt(args.fromBlock, 16) + 1;
      opts.widths.push(width);
      if (opts.failLogs?.()) throw new Error("connection not open on send()");
      if (opts.maxRange !== undefined && width > opts.maxRange) {
        opts.onReject?.();
        throw new Error("range too large");
      }
      return { logs: [] };
    },
  } as unknown as PluginLedgerConnectorBesu;
}

// Runs a watcher for the duration of `body` and always stops it, so a failing assertion cannot
// leave a poll loop running and hang the whole test process.
async function withWatcher(
  file: string,
  connector: PluginLedgerConnectorBesu,
  body: () => Promise<void>,
): Promise<void> {
  const gw = await gatewayServer();
  const watcher = newWatcher(file, gw.url, connector);
  const controller = new AbortController();
  watcher.start(controller.signal);
  try {
    await body();
  } finally {
    controller.abort();
    watcher.stop();
    await gw.close();
  }
}

test("catches up against a node that rejects wide log ranges", async () => {
  const file = await tmpFile();
  const widths: number[] = [];
  let caughtUp = false;

  await withWatcher(file, windowConnector({ head: () => 20_000, maxRange: 1_000, widths }), async () => {
    const store = new BlockWatermarkStore(file);
    caughtUp = await waitFor(async () => { await store.init(); return store.get(CONTRACT) === 20_000; }, 5_000);
  });

  assert.equal(caughtUp, true, "a fixed 5,000-block window against a 1,000-block limit never succeeds");
  assert.ok(Math.min(...widths) <= 1_000, "the window narrowed to something the node accepts");
});

// Without a ceiling, doubling after every success re-tests the limit on every other cycle:
// 512 ok, 1024 fail, 512 ok — half the cycles issue the request that drops the socket, and the
// failure streak resets on each success so the reconnect threshold never trips.
test("does not keep probing the rejected width once the node's limit is known", async () => {
  const file = await tmpFile();
  const widths: number[] = [];
  let head = 10_000;

  await withWatcher(file, windowConnector({ head: () => (head += 3_000), maxRange: 1_000, widths }), async () => {
    await sleep(1_500);
  });

  assert.ok(widths.length > 40, `expected sustained scanning, saw ${widths.length} requests`);
  // After the first few narrowing failures, almost every request must be one the node accepts: with
  // the ceiling, one request in fifty re-probes the limit (about 0.4%); without it, one in ten.
  const tail = widths.slice(Math.floor(widths.length / 2));
  const tailOver = tail.filter(w => w > 1_000).length;
  assert.ok(tailOver / tail.length < 0.03, `${tailOver} of ${tail.length} later requests were over the limit`);
});

// An absent node is not a range limit: every width fails when nothing answers, so the narrowest
// one reached says nothing about what the node can serve. A spoke that was simply down must not
// come back pinned at the floor and crawl through its catch-up.
test("restores the full window after an outage rather than treating it as a range limit", async () => {
  const file = await tmpFile();
  const widths: number[] = [];
  let down = true;
  let head = 200_000;
  let narrowedTo = Infinity;
  let restored = false;

  await withWatcher(file, windowConnector({
    head: () => (down ? head : (head += 50_000)),
    failLogs: () => down,
    widths,
  }), async () => {
    await waitFor(() => widths.length >= 12, 3_000);
    narrowedTo = Math.min(...widths);
    const duringOutage = widths.length;
    down = false; // the node comes back, with a lot to catch up
    restored = await waitFor(() => widths.slice(duringOutage).some(w => w >= 5_000), 5_000);
  });

  assert.equal(narrowedTo, 128, "the window narrows to its floor while nothing answers");
  assert.equal(restored, true, "the window must grow back to 5,000 instead of staying at the floor");
});

// A common failure mode: a request that is too wide drops the socket, and every
// call after it fails until the connection is back. A run of failures follows each wide request,
// so counting a streak of failures cannot tell this node from one that is absent. Only the log
// request failing at EVERY width, the floor included, identifies an absent node.
test("keeps the known ceiling when a wide request drops the socket", async () => {
  const file = await tmpFile();
  const widths: number[] = [];
  let socketDown = 0;
  let head = 10_000;
  const LIMIT = 600;

  await withWatcher(file, windowConnector({
    // The head moves a little on every read, so each poll has work and polls stay short: with a
    // head far away a single poll would walk the whole range and never reach the next one.
    head: () => {
      if (socketDown > 0) { socketDown--; throw new Error("connection not open on send()"); }
      return (head += 3_000);
    },
    maxRange: LIMIT,
    onReject: () => { socketDown = 3; },
    widths,
  }), async () => {
    await sleep(1_500);
  });

  const tail = widths.slice(Math.floor(widths.length / 2));
  const over = tail.filter(w => w > LIMIT).length;
  assert.ok(tail.length > 20, `expected sustained scanning, saw ${tail.length} requests in the tail`);
  // Count the over-limit requests rather than their share: each one drops the socket, and the
  // good polls issue many narrow requests that would dilute a ratio into looking harmless. A
  // re-probe now and then is by design; forgetting the ceiling after every drop is not.
  assert.ok(over <= 3, `${over} requests in the second half of the run were over the node's limit`);
});

// A head read that fails never reaches the log request, so there is no width to blame. The first
// request after the node is back must still be a full-width one.
test("leaves the window alone while the head read is what fails", async () => {
  const file = await tmpFile();
  const widths: number[] = [];
  let down = true;

  await withWatcher(file, windowConnector({
    head: () => {
      if (down) throw new Error("connection not open on send()");
      return 10_000_000;
    },
    widths,
  }), async () => {
    await sleep(200); // many failed polls
    down = false;
    await waitFor(() => widths.length > 0, 3_000);
  });

  assert.ok(widths.length > 0);
  assert.equal(widths[0], 5_000, "failed head reads must not narrow the scan window");
});

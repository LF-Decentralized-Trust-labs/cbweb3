// SPDX-License-Identifier: Apache-2.0
import { test } from "node:test";
import assert from "node:assert/strict";
import { promises as fs } from "fs";
import * as os from "os";
import * as path from "path";

import { BlockWatermarkStore } from "./block-watermark-store";

const silentLog = { info: () => {}, warn: () => {}, error: () => {} };

async function tmpFile(): Promise<string> {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), "lcr-watermark-test-"));
  return path.join(dir, "store.json");
}

test("BlockWatermarkStore returns undefined before any block is recorded", async () => {
  const store = new BlockWatermarkStore(await tmpFile(), silentLog);
  await store.init();
  assert.equal(store.get("lcr"), undefined);
});

test("BlockWatermarkStore persists and returns the last processed block per key", async () => {
  const store = new BlockWatermarkStore(await tmpFile(), silentLog);
  await store.init();

  await store.set("0xabc", 100);
  await store.set("0xdef", 250);
  assert.equal(store.get("0xabc"), 100);
  assert.equal(store.get("0xdef"), 250);

  await store.set("0xabc", 175);
  assert.equal(store.get("0xabc"), 175);
});

test("BlockWatermarkStore survives a restart — a fresh store reloads the persisted block", async () => {
  const file = await tmpFile();
  const first = new BlockWatermarkStore(file, silentLog);
  await first.init();
  await first.set("0xabc", 9999);

  const second = new BlockWatermarkStore(file, silentLog);
  await second.init();
  assert.equal(second.get("0xabc"), 9999);
});

test("BlockWatermarkStore persists the delivered set and chain meta across restart", async () => {
  const file = await tmpFile();
  const first = new BlockWatermarkStore(file, silentLog);
  await first.init();
  await first.markDelivered("0xtx:0");
  await first.setMeta("0xabc", "0xgenesis");
  assert.equal(first.hasDelivered("0xtx:0"), true);
  assert.equal(first.hasDelivered("0xtx:1"), false);

  const second = new BlockWatermarkStore(file, silentLog);
  await second.init();
  assert.equal(second.hasDelivered("0xtx:0"), true);
  assert.equal(second.getMeta("0xabc"), "0xgenesis");
});

test("BlockWatermarkStore.resetChain clears the delivered set and records new genesis", async () => {
  const file = await tmpFile();
  const store = new BlockWatermarkStore(file, silentLog);
  await store.init();
  await store.set("0xabc", 500);
  await store.markDelivered("0xtx:0");

  await store.resetChain("0xabc", 0, "0xnewgenesis");
  assert.equal(store.get("0xabc"), 0);
  assert.equal(store.hasDelivered("0xtx:0"), false, "delivered set dropped on chain reset");
  assert.equal(store.getMeta("0xabc"), "0xnewgenesis");
});

test("BlockWatermarkStore writes atomically (no leftover .tmp file)", async () => {
  const file = await tmpFile();
  const store = new BlockWatermarkStore(file, silentLog);
  await store.init();
  await store.set("0xabc", 42);
  assert.ok(await fs.stat(file), "final file exists");
  // Read the directory rather than stat one predicted name: a fixed `${file}.tmp` check passes
  // no matter what is left behind once temp names carry a per-instance suffix.
  const leftovers = (await fs.readdir(path.dirname(file))).filter(f => f !== path.basename(file));
  assert.deepEqual(leftovers, [], "no temp file left behind");
});

// Concurrent persistence. A watcher's writes are awaited one after another today, but the store
// is a shared durable file: persist() wrote to a fixed temp name with no serialisation, so any two
// writes in flight meant one rename ran first and the second found no temp file and threw ENOENT.
test("BlockWatermarkStore survives many concurrent writes without a single failure", async () => {
  const file = await tmpFile();
  const store = new BlockWatermarkStore(file, silentLog);
  await store.init();

  const failures: string[] = [];
  await Promise.all(Array.from({ length: 60 }, (_, i) =>
    store.set(`key-${i % 3}`, i).catch((e: Error) => failures.push(e.message))));

  assert.deepEqual(failures, [], "concurrent writes must not throw");
});

test("BlockWatermarkStore loses no state when writes of different kinds interleave", async () => {
  const file = await tmpFile();
  const store = new BlockWatermarkStore(file, silentLog);
  await store.init();

  await Promise.all([
    store.set("0xabc", 111),
    store.markDelivered("0xtx:0"),
    store.setMeta("0xabc", "0xgenesis"),
    store.set("0xdef", 222),
  ]);

  const reloaded = new BlockWatermarkStore(file, silentLog);
  await reloaded.init();
  assert.equal(reloaded.get("0xabc"), 111);
  assert.equal(reloaded.get("0xdef"), 222);
  assert.equal(reloaded.getMeta("0xabc"), "0xgenesis");
  assert.equal(reloaded.hasDelivered("0xtx:0"), true);
});

test("BlockWatermarkStore never has two writes in flight at once", async () => {
  const file = await tmpFile();
  const store = new BlockWatermarkStore(file, silentLog);
  await store.init();

  let inFlight = 0;
  let peak = 0;
  const realWriteFile = fs.writeFile.bind(fs);
  const original = fs.writeFile;
  (fs as { writeFile: unknown }).writeFile = async (...args: unknown[]) => {
    inFlight++;
    peak = Math.max(peak, inFlight);
    try {
      await new Promise(r => setTimeout(r, 2)); // widen the window a real disk would give
      return await (realWriteFile as (...a: unknown[]) => Promise<void>)(...args);
    } finally {
      inFlight--;
    }
  };
  try {
    await Promise.all(Array.from({ length: 12 }, (_, i) => store.set("0xabc", i)));
  } finally {
    (fs as { writeFile: unknown }).writeFile = original;
  }

  assert.equal(peak, 1, "writes must be serialised through a queue");
});

test("two BlockWatermarkStore instances on the same file do not collide", async () => {
  const file = await tmpFile();
  // Seed the file first, which is the production condition: it lives on a volume that outlives
  // the process, and an old process still shutting down can overlap a new one. When the file is
  // absent the first init() creates it and the two instances' write counters start one apart.
  await fs.writeFile(file, JSON.stringify({ blocks: {}, delivered: {}, meta: {} }), "utf8");
  const a = new BlockWatermarkStore(file, silentLog);
  const b = new BlockWatermarkStore(file, silentLog);
  await a.init();
  await b.init();

  const failures: string[] = [];
  await Promise.all(Array.from({ length: 40 }, (_, i) =>
    (i % 2 ? a : b).set(`key-${i}`, i).catch((e: Error) => failures.push(e.message))));

  assert.deepEqual(failures, [], "instances sharing one path must not throw ENOENT");
});

test("BlockWatermarkStore reads the legacy flat block-map shape", async () => {
  const file = await tmpFile();
  // A file written by the pre-R2-H-11 store: a flat {key: block} map.
  await fs.writeFile(file, JSON.stringify({ "0xabc": 777 }), "utf8");

  const store = new BlockWatermarkStore(file, silentLog);
  await store.init();
  assert.equal(store.get("0xabc"), 777, "legacy watermark preserved across upgrade");
});

// A write that fails part-way (a full disk) leaves a partly written temp file. Temp names are
// unique per write, so nothing later overwrites it: unless the failure path removes it, each
// failed write during a disk-pressure event leaves a full copy of the state behind and makes
// the shortage worse.
test("BlockWatermarkStore removes its temp file when a write fails part-way", async () => {
  const file = await tmpFile();
  const store = new BlockWatermarkStore(file, silentLog);
  await store.init();

  const realWriteFile = fs.writeFile.bind(fs);
  const original = fs.writeFile;
  (fs as { writeFile: unknown }).writeFile = async (...args: unknown[]) => {
    await (realWriteFile as (...a: unknown[]) => Promise<void>)(...args); // the partial file exists …
    throw new Error("ENOSPC: no space left on device");                    // … then the write fails
  };
  try {
    await assert.rejects(() => store.set("0xabc", 1), /ENOSPC/);
  } finally {
    (fs as { writeFile: unknown }).writeFile = original;
  }

  const leftovers = (await fs.readdir(path.dirname(file))).filter(f => f !== path.basename(file));
  assert.deepEqual(leftovers, [], "a failed write must not leave its temp file behind");
  // And the store keeps working: a failed write does not poison the queue.
  await store.set("0xabc", 2);
  assert.equal(store.get("0xabc"), 2);
});

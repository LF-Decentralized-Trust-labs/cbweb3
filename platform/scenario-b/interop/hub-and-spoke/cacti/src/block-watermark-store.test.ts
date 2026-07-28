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
  await assert.rejects(() => fs.stat(`${file}.tmp`), "temp file renamed away");
});

test("BlockWatermarkStore reads the legacy flat block-map shape", async () => {
  const file = await tmpFile();
  // A file written by the pre-R2-H-11 store: a flat {key: block} map.
  await fs.writeFile(file, JSON.stringify({ "0xabc": 777 }), "utf8");

  const store = new BlockWatermarkStore(file, silentLog);
  await store.init();
  assert.equal(store.get("0xabc"), 777, "legacy watermark preserved across upgrade");
});

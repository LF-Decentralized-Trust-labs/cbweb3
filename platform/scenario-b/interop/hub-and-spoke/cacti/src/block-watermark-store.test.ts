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

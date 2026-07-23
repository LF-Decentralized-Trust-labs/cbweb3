// SPDX-License-Identifier: Apache-2.0

import { promises as fs } from "fs";
import os from "os";
import path from "path";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { BlockWatermarkStore } from "./block-watermark-store";

const silentLog = { info: () => {}, warn: () => {}, error: () => {} };

describe("BlockWatermarkStore (R2-H-11)", () => {
  let dir: string;
  let file: string;

  beforeEach(async () => {
    dir = await fs.mkdtemp(path.join(os.tmpdir(), "lcr-watermark-test-"));
    file = path.join(dir, "store.json");
  });

  afterEach(async () => {
    await fs.rm(dir, { recursive: true, force: true });
  });

  it("returns undefined before any block is recorded", async () => {
    const store = new BlockWatermarkStore(file, silentLog);
    await store.init();
    expect(store.get("lcr")).toBeUndefined();
  });

  it("persists and returns the last processed block per key", async () => {
    const store = new BlockWatermarkStore(file, silentLog);
    await store.init();

    await store.set("0xabc", 100);
    await store.set("0xdef", 250);
    expect(store.get("0xabc")).toBe(100);
    expect(store.get("0xdef")).toBe(250);

    await store.set("0xabc", 175);
    expect(store.get("0xabc")).toBe(175);
  });

  it("survives a restart — a fresh store reloads the persisted block", async () => {
    const first = new BlockWatermarkStore(file, silentLog);
    await first.init();
    await first.set("0xabc", 9999);

    const second = new BlockWatermarkStore(file, silentLog);
    await second.init();
    expect(second.get("0xabc")).toBe(9999);
  });
});

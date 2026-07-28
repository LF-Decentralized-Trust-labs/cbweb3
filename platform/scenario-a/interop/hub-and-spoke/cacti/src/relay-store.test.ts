// SPDX-License-Identifier: Apache-2.0

import { promises as fs } from "fs";
import os from "os";
import path from "path";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { RelayStore } from "./relay-store";

const silentLog = { info: () => {}, warn: () => {}, error: () => {} };

describe("RelayStore block watermark (R2-H-11)", () => {
  let dir: string;
  let file: string;

  beforeEach(async () => {
    dir = await fs.mkdtemp(path.join(os.tmpdir(), "relay-store-test-"));
    file = path.join(dir, "store.json");
  });

  afterEach(async () => {
    await fs.rm(dir, { recursive: true, force: true });
  });

  it("returns undefined before any watermark is recorded", async () => {
    const store = new RelayStore(file, silentLog);
    await store.init();
    expect(store.getWatermark("spoke-a")).toBeUndefined();
  });

  it("persists and returns the last processed block per spoke", async () => {
    const store = new RelayStore(file, silentLog);
    await store.init();

    await store.setWatermark("spoke-a", 100);
    await store.setWatermark("spoke-b", 250);

    expect(store.getWatermark("spoke-a")).toBe(100);
    expect(store.getWatermark("spoke-b")).toBe(250);

    // Advancing the same spoke overwrites the previous value.
    await store.setWatermark("spoke-a", 175);
    expect(store.getWatermark("spoke-a")).toBe(175);
  });

  it("survives a restart — a fresh store reloads the persisted watermark", async () => {
    const first = new RelayStore(file, silentLog);
    await first.init();
    await first.setWatermark("spoke-a", 4242);

    // Simulate a process restart: a brand-new instance reading the same file.
    const second = new RelayStore(file, silentLog);
    await second.init();
    expect(second.getWatermark("spoke-a")).toBe(4242);
  });

  it("keeps delivered/retries state intact alongside the watermark", async () => {
    const store = new RelayStore(file, silentLog);
    await store.init();
    await store.markDelivered("trade-1:settle");
    await store.setWatermark("spoke-a", 900);

    const reloaded = new RelayStore(file, silentLog);
    await reloaded.init();
    expect(reloaded.hasDelivered("trade-1:settle")).toBe(true);
    expect(reloaded.getWatermark("spoke-a")).toBe(900);
  });

  it("persists HTLC dedup keys so a restart does not re-settle the same claim", async () => {
    const store = new RelayStore(file, silentLog);
    await store.init();
    // The relay records both the per-event guard and the echo guard on a successful settle.
    await store.markDelivered("htlc-evt:spoke-a:0xtx:0");
    await store.markDelivered("htlc-settled:spoke-b:contractB");

    const reloaded = new RelayStore(file, silentLog);
    await reloaded.init();
    expect(reloaded.hasDelivered("htlc-evt:spoke-a:0xtx:0")).toBe(true);
    expect(reloaded.hasDelivered("htlc-settled:spoke-b:contractB")).toBe(true);
    expect(reloaded.hasDelivered("htlc-evt:spoke-a:0xtx:1")).toBe(false);
  });

  it("persists chain meta (genesis hash) per spoke across restart", async () => {
    const store = new RelayStore(file, silentLog);
    await store.init();
    await store.setMeta("spoke-a", "0xgenesisA");

    const reloaded = new RelayStore(file, silentLog);
    await reloaded.init();
    expect(reloaded.getMeta("spoke-a")).toBe("0xgenesisA");
    expect(reloaded.getMeta("spoke-b")).toBeUndefined();
  });

  it("resetSpokeChain rewinds the watermark, records new genesis, and drops only that spoke's HTLC keys", async () => {
    const store = new RelayStore(file, silentLog);
    await store.init();
    await store.setWatermark("spoke-a", 500);
    await store.setMeta("spoke-a", "0xold");
    await store.markDelivered("htlc-evt:spoke-a:0xtx:0");
    await store.markDelivered("htlc-settled:spoke-a:cA");
    await store.markDelivered("htlc-evt:spoke-b:0xty:0"); // different spoke — must survive
    await store.markDelivered("trade-9:settle");          // FX key — must survive

    await store.resetSpokeChain("spoke-a", 0, "0xnew");

    expect(store.getWatermark("spoke-a")).toBe(0);
    expect(store.getMeta("spoke-a")).toBe("0xnew");
    expect(store.hasDelivered("htlc-evt:spoke-a:0xtx:0")).toBe(false);
    expect(store.hasDelivered("htlc-settled:spoke-a:cA")).toBe(false);
    expect(store.hasDelivered("htlc-evt:spoke-b:0xty:0")).toBe(true);
    expect(store.hasDelivered("trade-9:settle")).toBe(true);
  });

  it("writes atomically — no leftover .tmp file after a write", async () => {
    const store = new RelayStore(file, silentLog);
    await store.init();
    await store.setWatermark("spoke-a", 7);
    await expect(fs.stat(file)).resolves.toBeDefined();
    await expect(fs.stat(`${file}.tmp`)).rejects.toThrow();
  });

  it("reads a legacy file that has no meta field", async () => {
    // A store written before the meta field existed.
    await fs.writeFile(
      file,
      JSON.stringify({ delivered: { "k": 1 }, retries: [], watermarks: { "spoke-a": 88 } }),
      "utf8",
    );
    const store = new RelayStore(file, silentLog);
    await store.init();
    expect(store.getWatermark("spoke-a")).toBe(88);
    expect(store.hasDelivered("k")).toBe(true);
    expect(store.getMeta("spoke-a")).toBeUndefined();
  });
});

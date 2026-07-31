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

// The durable seq journal is the fix for finding 6 (cursor composition): the Go poller cursors on
// a stable seq that survives a relay restart, instead of a wall-clock ms regenerated on re-decode.
describe("RelayStore event journal (R2-H-11 cursor composition)", () => {
  let dir: string;
  let file: string;

  beforeEach(async () => {
    dir = await fs.mkdtemp(path.join(os.tmpdir(), "relay-journal-test-"));
    file = path.join(dir, "store.json");
  });

  afterEach(async () => {
    await fs.rm(dir, { recursive: true, force: true });
  });

  it("assigns a monotonic seq shared across kinds and serves events with seq > since", async () => {
    const store = new RelayStore(file, silentLog);
    await store.init();

    const s1 = await store.appendEvent("settle", "spoke-a:0x1:0", { contractId: "c1" });
    const l1 = await store.appendEvent("lock", "spoke-a:0x2:0", { contractId: "c2" });
    const s2 = await store.appendEvent("settle", "spoke-a:0x3:0", { contractId: "c3" });
    expect([s1, l1, s2]).toEqual([1, 2, 3]); // one monotonic counter across both kinds

    // getEventsSince(0) returns everything of that kind, in seq order, each carrying its seq.
    const settles = store.getEventsSince("settle", 0);
    expect(settles.map((e) => e["seq"])).toEqual([1, 3]);
    expect(settles.map((e) => e["contractId"])).toEqual(["c1", "c3"]);

    // A cursor past the first settle only yields the later one.
    expect(store.getEventsSince("settle", 1).map((e) => e["seq"])).toEqual([3]);
    expect(store.getEventsSince("settle", 3)).toEqual([]);
  });

  it("dedups by chain identity: a re-decoded event keeps its original seq and is not duplicated", async () => {
    const store = new RelayStore(file, silentLog);
    await store.init();

    const first = await store.appendEvent("settle", "spoke-a:0xtx:0", { contractId: "c1" });
    const again = await store.appendEvent("settle", "spoke-a:0xtx:0", { contractId: "c1" });
    expect(again).toBe(first); // same seq — this is what suppresses re-delivery after restart

    // Only one entry exists, and a fresh event still gets the NEXT seq (no seq reuse).
    expect(store.getEventsSince("settle", 0)).toHaveLength(1);
    const next = await store.appendEvent("settle", "spoke-a:0xtx:1", { contractId: "c2" });
    expect(next).toBe(first + 1);
  });

  it("survives a restart: journal entries and the seq counter persist", async () => {
    const first = new RelayStore(file, silentLog);
    await first.init();
    await first.appendEvent("settle", "spoke-a:0xtx:0", { contractId: "c1" });

    // Restart: a fresh instance reading the same file must resume the seq counter (not reuse 1)
    // and still serve the persisted event.
    const second = new RelayStore(file, silentLog);
    await second.init();
    expect(second.getEventsSince("settle", 0).map((e) => e["seq"])).toEqual([1]);
    const next = await second.appendEvent("settle", "spoke-a:0xother:0", { contractId: "c2" });
    expect(next).toBe(2); // monotonic across the restart — no collision with the persisted seq
  });

  it("resumes the seq counter past a legacy journal that has entries but no seq counter", async () => {
    // A hand-rolled file whose journal has a max seq of 7 but no top-level `seq` field.
    await fs.writeFile(
      file,
      JSON.stringify({
        delivered: {}, retries: [], watermarks: {}, meta: {},
        journal: { lock: [], settle: [{ seq: 7, id: "spoke-a:0x9:0", event: { contractId: "c9", seq: 7 } }] },
      }),
      "utf8",
    );
    const store = new RelayStore(file, silentLog);
    await store.init();
    const next = await store.appendEvent("settle", "spoke-a:0xnew:0", { contractId: "cN" });
    expect(next).toBe(8); // resumes at maxSeq+1, never reissuing 7
  });
});

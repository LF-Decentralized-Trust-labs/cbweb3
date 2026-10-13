// SPDX-License-Identifier: Apache-2.0

import { promises as fs } from "fs";
import os from "os";
import path from "path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

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

  // ── journal retention: what was dropped, so a consumer can tell it missed something ──
  //
  // The journal is capped, and a consumer that lags past the cap gets the entries that
  // survived and advances its cursor over the ones that did not — silently. For the settle
  // journal that is a lost settlement: the destination leg is the only party that can settle
  // itself, and the journal is how it finds out. The relay logs the trim; the consumer, which
  // is the one that loses, learns nothing.
  //
  // Contiguity cannot be used to spot the hole: `seq` is one counter shared by both journals,
  // so a kind's surviving seqs are legitimately non-contiguous. What is exact is the highest
  // seq ever dropped from that kind — a cursor at or below it has missed events.
  it("records the highest seq dropped from each journal, per kind", async () => {
    const store = new RelayStore(file, silentLog, 3); // tiny cap so the trim is reachable
    await store.init();

    expect(store.journalTrimmedThrough("settle")).toBe(0); // nothing dropped yet

    const seqs: number[] = [];
    for (let i = 0; i < 5; i++) {
      seqs.push(await store.appendEvent("settle", `s${i}`, { i }));
    }
    // 5 appended, cap 3 → the two oldest are gone.
    expect(store.journalTrimmedThrough("settle")).toBe(seqs[1]);
    expect(store.journalTrimmedThrough("lock")).toBe(0); // the other kind is untouched

    // Everything still retained is strictly above the mark, which is what makes the
    // comparison safe for a consumer.
    for (const e of store.getEventsSince("settle", 0)) {
      expect(e["seq"] as number).toBeGreaterThan(store.journalTrimmedThrough("settle"));
    }
  });

  it("keeps the trim mark across a restart", async () => {
    const store = new RelayStore(file, silentLog, 2);
    await store.init();
    for (let i = 0; i < 4; i++) await store.appendEvent("lock", `l${i}`, { i });
    const mark = store.journalTrimmedThrough("lock");
    expect(mark).toBeGreaterThan(0);

    const reloaded = new RelayStore(file, silentLog, 2);
    await reloaded.init();
    expect(reloaded.journalTrimmedThrough("lock")).toBe(mark);
  });

  // A store written by a relay that predates the mark must load, and must not claim events
  // were dropped when it simply does not know.
  it("reads a store with no trim mark as nothing dropped", async () => {
    await fs.writeFile(file, JSON.stringify({
      delivered: {}, retries: [], watermarks: {}, meta: {},
      journal: { lock: [], settle: [{ seq: 7, id: "x", event: { seq: 7 } }] }, seq: 8,
    }));
    const store = new RelayStore(file, silentLog);
    await store.init();
    expect(store.journalTrimmedThrough("settle")).toBe(0);
    expect(store.getEventsSince("settle", 0).length).toBe(1);
  });

  it("writes atomically — no leftover temp file after a write", async () => {
    const store = new RelayStore(file, silentLog);
    await store.init();
    await store.setWatermark("spoke-a", 7);
    await expect(fs.stat(file)).resolves.toBeDefined();
    // Read the directory rather than stat one predicted name. This asserted `${file}.tmp`, which
    // stopped being produced when temp names gained a per-instance suffix — so it passed no
    // matter what was left behind, which is worse than no test at all.
    const leftovers = (await fs.readdir(dir)).filter(f => f !== path.basename(file));
    expect(leftovers).toEqual([]);
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

// ── concurrent persistence ────────────────────────────────────────────────────
//
// Every spoke runs its own pollSpoke loop against ONE store instance, so writes are concurrent
// by design. persist() wrote to a fixed temp filename and renamed, with no serialisation: two
// writes in flight meant one renamed first and the second's rename found no temp file. That
// surfaces as
//
//   poll cycle error: ENOENT: no such file or directory,
//     rename '/data/cacti-relay-store.json.tmp' -> '/data/cacti-relay-store.json'
//
// and — because it was thrown inside the poll cycle — it aborted the cycle exactly as a chain
// failure did. That is the real damage: not corruption, but a store write that can stop the
// relay's work.
describe("RelayStore concurrent persistence", () => {
  let dir: string;
  let file: string;

  beforeEach(async () => {
    dir = await fs.mkdtemp(path.join(os.tmpdir(), "relay-store-concurrent-"));
    file = path.join(dir, "store.json");
  });
  afterEach(async () => {
    await fs.rm(dir, { recursive: true, force: true });
  });

  it("survives many concurrent writes without a single failure", async () => {
    const store = new RelayStore(file, silentLog);
    await store.init();

    const failures: string[] = [];
    const N = 60;
    await Promise.all(Array.from({ length: N }, (_, i) =>
      store.setWatermark(`spoke-${i % 3}`, i).catch((e: Error) => failures.push(e.message)),
    ));

    expect(failures).toEqual([]);
  });

  // Interleaved writes of different shapes must all be present at the end: the last write to
  // land has to carry every earlier one, not an older snapshot of the state.
  it("loses no state when writes of different kinds interleave", async () => {
    const store = new RelayStore(file, silentLog);
    await store.init();

    await Promise.all([
      store.setWatermark("spoke-a", 111),
      store.markDelivered("htlc-evt:spoke-a:0xtx:0"),
      store.setMeta("spoke-a", "0xgenesis"),
      store.setWatermark("spoke-b", 222),
      store.appendEvent("settle", "spoke-a:0xtx:0", { contractId: "c1" }),
    ]);

    const reloaded = new RelayStore(file, silentLog);
    await reloaded.init();
    expect(reloaded.getWatermark("spoke-a")).toBe(111);
    expect(reloaded.getWatermark("spoke-b")).toBe(222);
    expect(reloaded.getMeta("spoke-a")).toBe("0xgenesis");
    expect(reloaded.hasDelivered("htlc-evt:spoke-a:0xtx:0")).toBe(true);
    expect(reloaded.getEventsSince("settle", 0)).toHaveLength(1);
  });

  // Discriminates the QUEUE specifically: without it, writes to the same path overlap. Both
  // mechanisms happen to hide the ENOENT on their own, so assert the property each one is
  // actually for — this one is "one write at a time".
  it("never has two writes in flight at once", async () => {
    const store = new RelayStore(file, silentLog);
    await store.init();

    let inFlight = 0;
    let peak = 0;
    const realWriteFile = fs.writeFile.bind(fs);
    const spy = vi.spyOn(fs, "writeFile").mockImplementation(async (...args: unknown[]) => {
      inFlight++;
      peak = Math.max(peak, inFlight);
      try {
        await new Promise(r => setTimeout(r, 2)); // widen the window a real disk would give
        return await (realWriteFile as (...a: unknown[]) => Promise<void>)(...args);
      } finally {
        inFlight--;
      }
    });

    await Promise.all(Array.from({ length: 12 }, (_, i) => store.setWatermark("spoke-a", i)));
    spy.mockRestore();

    expect(peak).toBe(1);
  });

  // Discriminates the UNIQUE TEMP NAME specifically: the queue only orders writes made through
  // ONE instance. Two instances on the same path — an old process still shutting down while a
  // new one starts, sharing the relay volume — collide on a fixed temp name and one of them
  // throws ENOENT into its poll cycle.
  it("two store instances on the same file do not collide", async () => {
    // Seed the file first, which is the production condition: the store lives on a volume that
    // outlives the process. It also matters for what this test can see — when the file is
    // absent, the FIRST instance's init() creates it and consumes a write, so the two
    // instances' counters start one apart and never produce the same name. Seeded, both start
    // level, which is when a shared-component name actually collides.
    await fs.writeFile(file, JSON.stringify({
      delivered: {}, retries: [], watermarks: {}, meta: {},
      journal: { lock: [], settle: [] }, seq: 1,
    }), "utf8");

    const a = new RelayStore(file, silentLog);
    const b = new RelayStore(file, silentLog);
    await a.init();
    await b.init();

    // Hold each write open AFTER the bytes land and BEFORE the rename — that gap is where the
    // collision lives, so widening it makes the test decide rather than flip a coin. (Delaying
    // *before* writeFile does the opposite: the timers fire in order and the two instances end
    // up serialised, which is how an earlier version of this test passed against the bug.)
    const realWriteFile = fs.writeFile.bind(fs);
    const spy = vi.spyOn(fs, "writeFile").mockImplementation(async (...args: unknown[]) => {
      await (realWriteFile as (...a: unknown[]) => Promise<void>)(...args);
      await new Promise(r => setTimeout(r, 3));
    });

    // ONE write each, deliberately. Both instances are then on their first write, so a temp
    // name built from anything they share — a pid and a per-instance counter, say — is the same
    // string for both. Ten writes each would let their counters drift apart and the collision
    // would land only sometimes, which is how this bug behaves in production and exactly what a
    // test must not reproduce.
    const failures: string[] = [];
    await Promise.all([
      a.setWatermark("spoke-a", 1).catch((e: Error) => failures.push(e.message)),
      b.setWatermark("spoke-b", 2).catch((e: Error) => failures.push(e.message)),
    ]);
    spy.mockRestore();

    expect(failures).toEqual([]);
  });

  // The temp file is an implementation detail that must never be left behind, and never shared
  // between two writes in flight.
  it("leaves no temp file behind after concurrent writes", async () => {
    const store = new RelayStore(file, silentLog);
    await store.init();
    await Promise.all(Array.from({ length: 20 }, (_, i) => store.setWatermark("spoke-a", i)));

    const leftovers = (await fs.readdir(dir)).filter(f => f !== "store.json");
    expect(leftovers).toEqual([]);
  });
});

// SPDX-License-Identifier: Apache-2.0
//
// The relay's poll cycle runs several independent responsibilities. They must not take each
// other down.
//
// An FX agreement created on one spoke never reached another. The relay was running, the
// destination was registered, the dedup guard had not skipped it — and the forwarding step had
// never been attempted. Every step of a cycle shared one `try`, so a failing Besu connection
// aborted the cycle before it reached FX forwarding, which is REST in, gRPC out and needs no
// chain at all. Nothing crossed spokes while it lasted, and nothing named a cause.

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { mkdtempSync, rmSync } from "node:fs";
import { join, resolve } from "node:path";
import { tmpdir } from "node:os";
import { HtlcRelay } from "./htlc-relay";
import { RelayStore } from "./relay-store";

const PROTO = resolve(__dirname, "../../../../apis/proto/payment_orchestrator/v1/payment_orchestrator.proto");

const SPOKE = {
  id: "spoke-costa-rica", besuRpc: "http://cr", besuWs: "ws://cr", htlcAddress: "0xaaaa",
  internalApiUrl: "http://cr:18080", grpcEndpoint: "cr:1",
};

describe("poll cycle fault isolation", () => {
  let dir: string;
  let file: string;
  let fetchCalls: string[];

  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), "poll-isolation-"));
    file = join(dir, "store.json");
    fetchCalls = [];
    // pollFXAgreementsRest's first act is fetch(internalApiUrl) — our probe for "it was reached".
    vi.stubGlobal("fetch", vi.fn(async (u: string) => {
      fetchCalls.push(String(u));
      return { ok: false, status: 503 };
    }));
  });
  afterEach(() => {
    rmSync(dir, { recursive: true, force: true });
    vi.unstubAllGlobals();
  });

  // The chain is unreachable: the connection is dropped and every call on it throws.
  function deadChain() {
    return {
      async getBlock(req: { blockHashOrBlockNumber: string | number }) {
        if (req.blockHashOrBlockNumber === 0) return { block: { number: 0, hash: "0xg" } };
        throw new Error("connection not open on send()");
      },
      async getPastLogs() { throw new Error("connection not open on send()"); },
      async shutdown() {},
    };
  }

  function liveChain(head = 10) {
    return {
      async getBlock(req: { blockHashOrBlockNumber: string | number }) {
        if (req.blockHashOrBlockNumber === 0) return { block: { number: 0, hash: "0xg" } };
        return { block: { number: head } };
      },
      async getPastLogs() { return { logs: [] }; },
      async shutdown() {},
    };
  }

  async function run(store: RelayStore, connector: unknown, ms: number) {
    const relay = new HtlcRelay([SPOKE] as any, PROTO, 5, "secret", store as any,
      new Map<string, any>([[SPOKE.id, connector]]));
    const logs = { warn: [] as string[], error: [] as string[] };
    (relay as any).log = {
      info: () => {}, warn: (m: string) => logs.warn.push(m), error: (m: string) => logs.error.push(m),
    };
    const controller = new AbortController();
    (relay as any).signal = controller.signal;
    void (relay as any).pollSpoke(SPOKE, controller.signal);
    await new Promise(r => setTimeout(r, ms));
    controller.abort();
    await new Promise(r => setTimeout(r, 20));
    return { relay, logs };
  }

  it("forwards FX agreements even while the chain is unreachable", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();

    const { logs } = await run(store, deadChain(), 300);

    // The chain failure is still reported — this is isolation, not suppression.
    expect(logs.warn.join("\n")).toMatch(/connection not open on send/);
    // And the step that never needed the chain ran anyway.
    expect(fetchCalls.length).toBeGreaterThan(0);
    expect(fetchCalls[0]).toContain("/internal/v1/payments/fx/agreements");
  }, 15_000);

  // The isolation has to hold on EVERY path out of the scan, not just the throwing one. When a
  // spoke's head sits at or below the watermark — a node restored from an older data dir, a
  // chain rewound under the same genesis, block production stopped with RPC still answering —
  // the scan has nothing to do and says so. That must not take FX transport with it, which is
  // the same coupling this change exists to remove, on a quieter path.
  it("forwards FX agreements while the chain head is behind the watermark", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    await store.setWatermark(SPOKE.id, 5_000); // far ahead of the head below

    const rewoundChain = {
      async getBlock(req: { blockHashOrBlockNumber: string | number }) {
        if (req.blockHashOrBlockNumber === 0) return { block: { number: 0, hash: "0xg" } };
        return { block: { number: 10 } };
      },
      async getPastLogs() { return { logs: [] }; },
      async shutdown() {},
    };

    const { logs } = await run(store, rewoundChain, 300);

    expect(logs.warn.join("\n")).toMatch(/is behind resume block/);
    // Many cycles ran, and FX transport must have been attempted on each of them — not once.
    expect(fetchCalls.length).toBeGreaterThan(3);
  }, 15_000);

  // The converse: a failing FX endpoint must not stop chain observation.
  it("keeps scanning the chain while the FX endpoint is failing", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    vi.stubGlobal("fetch", vi.fn(async () => { throw new Error("ECONNREFUSED"); }));

    await run(store, liveChain(10), 300);

    // The watermark advanced despite FX being down.
    expect(store.getWatermark(SPOKE.id)).toBe(10);
  }, 15_000);

  // The stall alarm must survive the very failure it exists to report. It used to sit inside
  // the block-scan try, after getBlock — so when getBlock was what threw, the alarm meant to
  // catch a stopped scan was skipped by the stop.
  it("reports a stalled scan even when getBlock is what throws", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    await store.setWatermark(SPOKE.id, 100);

    const relay = new HtlcRelay([SPOKE] as any, PROTO, 5, "secret", store as any,
      new Map<string, any>([[SPOKE.id, deadChain()]]));
    const errors: string[] = [];
    (relay as any).log = { info: () => {}, warn: () => {}, error: (m: string) => errors.push(m) };
    // Last advance an hour ago — well past the ten-minute threshold.
    (relay as any).watermarkAdvancedAt.set(SPOKE.id, Date.now() - 3_600_000);

    const controller = new AbortController();
    (relay as any).signal = controller.signal;
    void (relay as any).pollSpoke(SPOKE, controller.signal);
    await new Promise(r => setTimeout(r, 300));
    controller.abort();
    await new Promise(r => setTimeout(r, 20));

    const stall = errors.filter(e => /not advanced/i.test(e));
    expect(stall).toHaveLength(1);
    // With the chain unreachable the head is unknown, and the message must say so rather than
    // print a stale or invented number.
    expect(stall[0]).toMatch(/unknown/i);
  }, 15_000);

  // A relay that keeps advancing must never page anyone. (An earlier version of this test
  // pre-set the last-advance stamp to an hour ago while the chain was healthy — a state a
  // healthy relay cannot be in, since every advance refreshes that stamp. It asserted the
  // wrong thing.)
  it("never reports a stall while the scan keeps advancing", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    await store.setWatermark(SPOKE.id, 1);

    // A head that keeps moving, so every cycle has something to scan and advances.
    let head = 10;
    const movingChain = {
      async getBlock(req: { blockHashOrBlockNumber: string | number }) {
        if (req.blockHashOrBlockNumber === 0) return { block: { number: 0, hash: "0xg" } };
        head += 5;
        return { block: { number: head } };
      },
      async getPastLogs() { return { logs: [] }; },
      async shutdown() {},
    };

    const { logs } = await run(store, movingChain, 300);

    expect(logs.error.filter(e => /not advanced/i.test(e))).toHaveLength(0);
    expect(store.getWatermark(SPOKE.id)).toBeGreaterThan(1);
  }, 15_000);
});

// ── surviving a catch-up ──────────────────────────────────────────────────────
//
// A spoke can fall far behind the head after its Besu container restarts under a longer-lived
// relay. Each cycle then issued a getPastLogs over a fixed
// 5,000-block window, which dropped the freshly reconnected WebSocket; the relay's reconnect
// ran, reported success, and failed again on the next cycle. It could not get out on its own —
// an operator had to restart the container. A request that is too big to succeed must not be
// repeated unchanged for ever.
describe("catch-up is survivable", () => {
  let dir: string;
  let file: string;

  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), "catchup-"));
    file = join(dir, "store.json");
    vi.stubGlobal("fetch", vi.fn(async () => ({ ok: false, status: 503 })));
  });
  afterEach(() => {
    rmSync(dir, { recursive: true, force: true });
    vi.unstubAllGlobals();
  });

  // A connector that fails any getPastLogs wider than `limit` blocks — the shape of a node that
  // cannot serve a heavy range over its websocket.
  function rangeLimitedChain(limit: number, head: number) {
    const widths: number[] = [];
    return {
      widths,
      async getBlock(req: { blockHashOrBlockNumber: string | number }) {
        if (req.blockHashOrBlockNumber === 0) return { block: { number: 0, hash: "0xg" } };
        return { block: { number: head } };
      },
      async getPastLogs(args: { fromBlock: number; toBlock: number }) {
        const width = args.toBlock - args.fromBlock + 1;
        widths.push(width);
        if (width > limit) throw new Error("connection not open on send()");
        return { logs: [] };
      },
      async shutdown() {},
    };
  }

  it("narrows the scan window until the chain can serve it, then catches up", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    await store.setWatermark(SPOKE.id, 0); // 120k blocks behind

    const chain = rangeLimitedChain(600, 120_000);
    const relay = new HtlcRelay([SPOKE] as any, PROTO, 5, "secret", store as any,
      new Map<string, any>([[SPOKE.id, chain]]));
    (relay as any).log = { info: () => {}, warn: () => {}, error: () => {} };

    const controller = new AbortController();
    (relay as any).signal = controller.signal;
    void (relay as any).pollSpoke(SPOKE, controller.signal);
    await new Promise(r => setTimeout(r, 600));
    controller.abort();
    await new Promise(r => setTimeout(r, 20));

    // It backed off to a width the node accepts …
    expect(Math.min(...chain.widths)).toBeLessThanOrEqual(600);
    // … and made real progress without anyone restarting it.
    expect(store.getWatermark(SPOKE.id)).toBeGreaterThan(0);
  }, 20_000);

  // A node that was DOWN is not a node that chokes on wide ranges, and the two must not be
  // treated alike. Observed live: stopping a spoke's Besu narrowed the window all the way to the
  // floor — every width fails when there is nothing answering — and after the node came back the
  // window stayed at the floor, because the narrowest failure had been recorded as if it were a
  // range limit. A spoke 120k blocks behind then crawls for the best part of an hour at 128
  // blocks a cycle instead of catching up in a minute, which is the very situation the adaptive
  // window exists for. A genuine range limit fails one cycle at a time; an absent node fails in a streak.
  it("restores the full window after an outage, rather than treating it as a range limit", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    await store.setWatermark(SPOKE.id, 0);

    let down = true;
    const widths: number[] = [];
    const outageThenFine = {
      async getBlock(req: { blockHashOrBlockNumber: string | number }) {
        if (req.blockHashOrBlockNumber === 0) return { block: { number: 0, hash: "0xg" } };
        if (down) throw new Error("connection not open on send()");
        return { block: { number: 10_000_000 } };
      },
      async getPastLogs(args: { fromBlock: number; toBlock: number }) {
        if (down) throw new Error("connection not open on send()");
        widths.push(args.toBlock - args.fromBlock + 1);
        return { logs: [] };
      },
      async shutdown() {},
    };

    // WITH a connectorFactory, because the relay has one in production and it changes the
    // outcome: the reconnect zeroes the failure counter the moment it succeeds. A test without
    // one leaves that counter climbing and passes against an implementation that reads it —
    // which is how the first version of this fix passed here and did nothing on a live stack.
    const relay = new HtlcRelay([SPOKE] as any, PROTO, 5, "secret", store as any,
      new Map<string, any>([[SPOKE.id, outageThenFine]]),
      async () => outageThenFine as any);
    (relay as any).log = { info: () => {}, warn: () => {}, error: () => {} };

    const controller = new AbortController();
    (relay as any).signal = controller.signal;
    void (relay as any).pollSpoke(SPOKE, controller.signal);

    await new Promise(r => setTimeout(r, 300)); // long enough to reach the floor
    down = false;
    await new Promise(r => setTimeout(r, 400)); // and to recover
    controller.abort();
    await new Promise(r => setTimeout(r, 20));

    // Back to full width, not pinned at the floor.
    expect(widths[widths.length - 1]).toBe(5_000);
  }, 20_000);

  // Growing back must not mean re-testing the ceiling on every single cycle. Against a node
  // with a fixed log-range limit, doubling after every success oscillates across it for ever
  // (512 ok → 1024 fail → 512 ok → …): half the cycles issue the heavy request that drops the
  // socket, and because `failures` resets on each success the reconnect threshold never trips.
  // Settle at a width the node actually serves.
  it("settles below a fixed range ceiling instead of oscillating across it", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    await store.setWatermark(SPOKE.id, 0);

    const chain = rangeLimitedChain(600, 10_000_000);
    const relay = new HtlcRelay([SPOKE] as any, PROTO, 5, "secret", store as any,
      new Map<string, any>([[SPOKE.id, chain]]));
    (relay as any).log = { info: () => {}, warn: () => {}, error: () => {} };

    const controller = new AbortController();
    (relay as any).signal = controller.signal;
    void (relay as any).pollSpoke(SPOKE, controller.signal);
    await new Promise(r => setTimeout(r, 900));
    controller.abort();
    await new Promise(r => setTimeout(r, 20));

    // Look only at the tail: the early cycles legitimately probe downwards.
    const tail = chain.widths.slice(-20);
    const overCeiling = tail.filter(w => w > 600).length;
    expect(tail.length).toBeGreaterThan(10);
    // A handful of probes is fine; half the cycles failing is the oscillation.
    expect(overCeiling / tail.length).toBeLessThan(0.25);
  }, 25_000);

  // Backing off must not become the new normal: once the node copes, the window has to grow
  // again, or a spoke that fell far behind would crawl for ever at the minimum width.
  it("widens the window again once the chain copes", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    await store.setWatermark(SPOKE.id, 0);

    // A head far enough away that the relay never finishes catching up inside the test: the
    // recorded widths then stay the adaptive width, instead of ending on a truncated remainder
    // once fromBlock approaches the head (which is what an earlier version of this test
    // measured, and why it compared a remainder against the minimum).
    const FAR_HEAD = 10_000_000;
    // Fails the first few wide requests, then serves anything — a node that has recovered.
    let failsLeft = 3;
    const widths: number[] = [];
    const flakyThenFine = {
      async getBlock(req: { blockHashOrBlockNumber: string | number }) {
        if (req.blockHashOrBlockNumber === 0) return { block: { number: 0, hash: "0xg" } };
        return { block: { number: FAR_HEAD } };
      },
      async getPastLogs(args: { fromBlock: number; toBlock: number }) {
        widths.push(args.toBlock - args.fromBlock + 1);
        if (failsLeft-- > 0) throw new Error("connection not open on send()");
        return { logs: [] };
      },
      async shutdown() {},
    };

    const relay = new HtlcRelay([SPOKE] as any, PROTO, 5, "secret", store as any,
      new Map<string, any>([[SPOKE.id, flakyThenFine]]));
    (relay as any).log = { info: () => {}, warn: () => {}, error: () => {} };

    const controller = new AbortController();
    (relay as any).signal = controller.signal;
    void (relay as any).pollSpoke(SPOKE, controller.signal);
    await new Promise(r => setTimeout(r, 600));
    controller.abort();
    await new Promise(r => setTimeout(r, 20));

    const narrowest = Math.min(...widths);
    const lastWidth = widths[widths.length - 1];
    expect(lastWidth).toBeGreaterThan(narrowest);
  }, 20_000);
});

// ── the window tracks the node's range limit, and nothing else ───────────────────────────────
//
// The window exists to find the widest getPastLogs a node can serve. Only a failed getPastLogs
// says anything about that. A getBlock that times out, or a store write that fails, narrows
// nothing a node could be blamed for — and a node that drops its socket on a wide request fails
// every call that follows until it reconnects, so a run of failures is the NORMAL aftermath of a
// range limit, not evidence that the node was absent.
describe("scan window adapts to the range limit only", () => {
  let dir: string;
  let file: string;
  const quiet = { info: () => {}, warn: () => {}, error: () => {} };

  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), "window-"));
    file = join(dir, "store.json");
    vi.stubGlobal("fetch", vi.fn(async () => ({ ok: false, status: 503 })));
  });
  afterEach(() => {
    rmSync(dir, { recursive: true, force: true });
    vi.unstubAllGlobals();
  });

  async function runFor(store: RelayStore, chain: unknown, ms: number) {
    const relay = new HtlcRelay([SPOKE] as any, PROTO, 5, "secret", store as any,
      new Map<string, any>([[SPOKE.id, chain]]));
    (relay as any).log = quiet;
    const controller = new AbortController();
    (relay as any).signal = controller.signal;
    void (relay as any).pollSpoke(SPOKE, controller.signal);
    await new Promise(r => setTimeout(r, ms));
    controller.abort();
    await new Promise(r => setTimeout(r, 20));
  }

  // A common failure mode: a request that is too wide drops the socket, and
  // every call after it fails until the connection is back. A run of failures follows each wide
  // request, so counting a streak of failures cannot tell this node from one that is absent.
  it("keeps the known ceiling when a wide request drops the socket", async () => {
    const store = new RelayStore(file, quiet);
    await store.init();
    await store.setWatermark(SPOKE.id, 0);

    const widths: number[] = [];
    let socketDown = 0;
    const LIMIT = 600;
    const chain = {
      async getBlock(req: { blockHashOrBlockNumber: string | number }) {
        if (req.blockHashOrBlockNumber === 0) return { block: { number: 0, hash: "0xg" } };
        if (socketDown > 0) { socketDown--; throw new Error("connection not open on send()"); }
        return { block: { number: 10_000_000 } };
      },
      async getPastLogs(args: { fromBlock: number; toBlock: number }) {
        const width = args.toBlock - args.fromBlock + 1;
        widths.push(width);
        if (width > LIMIT) { socketDown = 3; throw new Error("connection not open on send()"); }
        return { logs: [] };
      },
      async shutdown() {},
    };

    await runFor(store, chain, 1_200);

    const tail = widths.slice(Math.floor(widths.length / 2));
    const over = tail.filter(w => w > LIMIT).length;
    expect(tail.length).toBeGreaterThan(20);
    // A re-probe now and then is by design; forgetting the ceiling after every drop is not.
    expect(over / tail.length).toBeLessThan(0.05);
  }, 20_000);

  // A scan failure that is not the log request — here the watermark write — must not be read as
  // a range limit.
  it("does not narrow the window for a failure that is not getPastLogs", async () => {
    const store = new RelayStore(file, quiet);
    await store.init();
    await store.setWatermark(SPOKE.id, 0);
    const failWrites = vi.spyOn(store, "setWatermark");
    failWrites.mockRejectedValueOnce(new Error("ENOSPC: no space left on device"));
    failWrites.mockRejectedValueOnce(new Error("ENOSPC: no space left on device"));
    failWrites.mockRejectedValueOnce(new Error("ENOSPC: no space left on device"));

    const widths: number[] = [];
    const chain = {
      async getBlock(req: { blockHashOrBlockNumber: string | number }) {
        if (req.blockHashOrBlockNumber === 0) return { block: { number: 0, hash: "0xg" } };
        return { block: { number: 10_000_000 } };
      },
      async getPastLogs(args: { fromBlock: number; toBlock: number }) {
        widths.push(args.toBlock - args.fromBlock + 1);
        return { logs: [] };
      },
      async shutdown() {},
    };

    await runFor(store, chain, 300);

    expect(widths.length).toBeGreaterThan(4);
    expect(Math.min(...widths)).toBe(5_000); // never narrowed: the node served every request
  }, 20_000);

  // An outage seen as a failing getBlock never reaches getPastLogs, so there is no width to
  // blame. The first request after the node is back must still be a full-width one.
  it("leaves the window alone while getBlock is what fails", async () => {
    const store = new RelayStore(file, quiet);
    await store.init();
    await store.setWatermark(SPOKE.id, 0);

    let down = true;
    const widths: number[] = [];
    const chain = {
      async getBlock(req: { blockHashOrBlockNumber: string | number }) {
        if (req.blockHashOrBlockNumber === 0) return { block: { number: 0, hash: "0xg" } };
        if (down) throw new Error("connection not open on send()");
        return { block: { number: 10_000_000 } };
      },
      async getPastLogs(args: { fromBlock: number; toBlock: number }) {
        widths.push(args.toBlock - args.fromBlock + 1);
        return { logs: [] };
      },
      async shutdown() {},
    };

    const relay = new HtlcRelay([SPOKE] as any, PROTO, 5, "secret", store as any,
      new Map<string, any>([[SPOKE.id, chain]]));
    (relay as any).log = quiet;
    const controller = new AbortController();
    (relay as any).signal = controller.signal;
    void (relay as any).pollSpoke(SPOKE, controller.signal);
    await new Promise(r => setTimeout(r, 200)); // many failed cycles
    down = false;
    await new Promise(r => setTimeout(r, 200));
    controller.abort();
    await new Promise(r => setTimeout(r, 20));

    expect(widths.length).toBeGreaterThan(0);
    expect(widths[0]).toBe(5_000);
  }, 20_000);

  // The converse: when the LOG request is what fails for a node that is simply not answering, it
  // fails at every width including the floor, which a real range limit never does. That is the
  // signature of an absent node, and once it answers again the full window must come back.
  it("restores the full window once a node that failed even at the floor answers again", async () => {
    const store = new RelayStore(file, quiet);
    await store.init();
    await store.setWatermark(SPOKE.id, 0);

    let down = true;
    const widths: number[] = [];
    const chain = {
      async getBlock(req: { blockHashOrBlockNumber: string | number }) {
        if (req.blockHashOrBlockNumber === 0) return { block: { number: 0, hash: "0xg" } };
        return { block: { number: 10_000_000 } };
      },
      async getPastLogs(args: { fromBlock: number; toBlock: number }) {
        widths.push(args.toBlock - args.fromBlock + 1);
        if (down) throw new Error("connection not open on send()");
        return { logs: [] };
      },
      async shutdown() {},
    };

    const relay = new HtlcRelay([SPOKE] as any, PROTO, 5, "secret", store as any,
      new Map<string, any>([[SPOKE.id, chain]]));
    (relay as any).log = quiet;
    const controller = new AbortController();
    (relay as any).signal = controller.signal;
    void (relay as any).pollSpoke(SPOKE, controller.signal);
    await new Promise(r => setTimeout(r, 300));
    const floor = Math.min(...widths);
    down = false;
    await new Promise(r => setTimeout(r, 400));
    controller.abort();
    await new Promise(r => setTimeout(r, 20));

    expect(floor).toBe(128);
    expect(widths[widths.length - 1]).toBe(5_000);
  }, 20_000);
});

// ── a healthy but slow cycle is not a failing connection ─────────────────────────────────────
//
// /health reports the chain head only while it still means something. The cycle now includes the
// FX transport, whose REST and gRPC calls have their own timeouts, so the gap between two head
// reads is the scan plus that plus the poll interval — often far over three poll intervals. A
// head that is old because the cycle is slow must not be reported as unknown: nothing is failing,
// and the dashboard would page on a connection that is fine.
describe("head freshness under a slow cycle", () => {
  let dir: string;
  let file: string;

  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), "freshness-"));
    file = join(dir, "store.json");
    // The FX endpoint answers, slowly: each cycle spends ~150ms there.
    vi.stubGlobal("fetch", vi.fn(async () => {
      await new Promise(r => setTimeout(r, 150));
      return { ok: false, status: 503 };
    }));
  });
  afterEach(() => {
    rmSync(dir, { recursive: true, force: true });
    vi.unstubAllGlobals();
  });

  it("keeps reporting a head that was read in the last cycle while FX transport is slow", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    await store.setWatermark(SPOKE.id, 900);

    const chain = {
      async getBlock(req: { blockHashOrBlockNumber: string | number }) {
        if (req.blockHashOrBlockNumber === 0) return { block: { number: 0, hash: "0xg" } };
        return { block: { number: 1_000 } };
      },
      async getPastLogs() { return { logs: [] }; },
      async shutdown() {},
    };
    const relay = new HtlcRelay([SPOKE] as any, PROTO, 5, "secret", store as any,
      new Map<string, any>([[SPOKE.id, chain]]));
    (relay as any).log = { info: () => {}, warn: () => {}, error: () => {} };
    const controller = new AbortController();
    (relay as any).signal = controller.signal;
    // Registered the way addSpoke registers a spoke: getScanLiveness reports what is being watched.
    (relay as any).watching.add(SPOKE.id);
    void (relay as any).pollSpoke(SPOKE, controller.signal);

    await new Promise(r => setTimeout(r, 80)); // the first head has been read
    const heads: Array<number | null> = [];
    for (let i = 0; i < 20; i++) {
      heads.push(relay.getScanLiveness().find(r => r.spokeId === SPOKE.id)?.head ?? null);
      await new Promise(r => setTimeout(r, 30));
    }
    controller.abort();
    await new Promise(r => setTimeout(r, 200));

    expect(heads.filter(h => h === null)).toEqual([]);
  }, 20_000);
});

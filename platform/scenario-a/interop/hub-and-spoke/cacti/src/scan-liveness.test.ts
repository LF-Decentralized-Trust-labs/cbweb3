// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { mkdtempSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { HtlcRelay } from "./htlc-relay";
import { resolve } from "node:path";
import { RelayStore } from "./relay-store";

// The number that mattered was the one nobody watched.
//
// Through the whole LNET outage the relay looked healthy: it logged continuously, answered its
// health probe, and had not crashed. What it had stopped doing was advancing its block
// watermark, and that was visible only by reading a JSON file out of a Docker volume twice,
// thirty seconds apart. Three days passed. The runbook's first check should be a curl, and a
// scan that has stopped should say so itself rather than wait to be asked.
// The real proto: addSpoke builds a gRPC client from it, so a fake path would make these tests
// avoid the very call path that a live relay takes when the toolkit registers a spoke.
const PROTO = resolve(__dirname, "../../../../apis/proto/payment_orchestrator/v1/payment_orchestrator.proto");

describe("chain-scan liveness is observable", () => {
  let dir: string;
  let file: string;

  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), "scan-liveness-"));
    file = join(dir, "store.json");
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 503 }));
  });
  afterEach(() => {
    rmSync(dir, { recursive: true, force: true });
    vi.unstubAllGlobals();
  });

  // Registers the spoke the way a deployment does: an empty constructor list, then addSpoke
  // from the relay's POST /api/v1/spokes handler.
  async function relayWith(store: RelayStore) {
    const relay = new HtlcRelay([], PROTO, 5, "secret", store as any, new Map(),
      async () => ({
        async getBlock() { return { block: { number: 0, hash: "0xg" } }; },
        async getPastLogs() { return { logs: [] }; },
        async shutdown() {},
      }) as any);
    (relay as any).log = { info: () => {}, warn: () => {}, error: () => {} };
    const controller = new AbortController();
    relay.start(controller.signal);
    await relay.addSpoke({
      id: "spoke-a", besuRpc: "http://a", besuWs: "ws://a", htlcAddress: "0xaaaa",
      internalApiUrl: "http://a:1", grpcEndpoint: "a:1",
    } as any);
    controller.abort();
    return relay;
  }

  it("reports each spoke's watermark, head and lag", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    await store.setWatermark("spoke-a", 900);

    const relay = await relayWith(store);
    (relay as any).chainHeads.set("spoke-a", 1000);

    const report = relay.getScanLiveness();
    const a = report.find(r => r.spokeId === "spoke-a");
    expect(a).toBeDefined();
    expect(a!.watermark).toBe(900);
    expect(a!.head).toBe(1000);
    expect(a!.blocksBehind).toBe(100);
  });

  // In a real deployment NO spoke comes from the constructor: the toolkit registers each one at
  // runtime with POST /api/v1/spokes, and addSpoke starts a watcher without touching the
  // constructor list. Reporting only the constructor list therefore reported nothing at all,
  // and an empty `scan` on a live relay reads as "no spokes to worry about" — the same
  // reassuring silence this endpoint exists to break. Found by running it, not by a unit test:
  // every test here had handed the relay its spokes up front.
  it("reports spokes registered at runtime, not only those passed to the constructor", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    await store.setWatermark("spoke-late", 42);

    const relay = new HtlcRelay([], PROTO, 5, "secret", store as any, new Map(),
      async () => ({
        async getBlock() { return { block: { number: 42, hash: "0xg" } }; },
        async getPastLogs() { return { logs: [] }; },
        async shutdown() {},
      }) as any);
    (relay as any).log = { info: () => {}, warn: () => {}, error: () => {} };

    const controller = new AbortController();
    relay.start(controller.signal);
    await relay.addSpoke({
      id: "spoke-late", besuRpc: "http://l", besuWs: "ws://l", htlcAddress: "0xcccc",
      internalApiUrl: "http://l:1", grpcEndpoint: "l:1",
    } as any);
    controller.abort();

    const report = relay.getScanLiveness();
    expect(report.map(r => r.spokeId)).toContain("spoke-late");
    expect(report.find(r => r.spokeId === "spoke-late")!.watermark).toBe(42);
  }, 15_000);

  // A spoke registered but never scanned is not "fine, at zero" — it is unknown, and saying
  // zero would read as healthy on a dashboard.
  it("reports a spoke that has never scanned as having no watermark", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();

    const report = (await relayWith(store)).getScanLiveness();
    const a = report.find(r => r.spokeId === "spoke-a");
    expect(a!.watermark).toBeNull();
  });

  // The age is what separates "catching up" from "stopped". A relay 5,000 blocks behind and
  // moving is working; one 3 blocks behind and frozen for an hour is the outage.
  it("reports how long it has been since the watermark last advanced", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    await store.setWatermark("spoke-a", 10);

    const relay = await relayWith(store);
    (relay as any).watermarkAdvancedAt.set("spoke-a", Date.now() - 120_000);

    const a = relay.getScanLiveness().find(r => r.spokeId === "spoke-a");
    expect(a!.secondsSinceAdvance).toBeGreaterThanOrEqual(119);
  });
});

// A component that is busy is not a component that is working. The relay must say when its
// own scan has stopped — that is the sentence nobody got for three days.
describe("a stalled scan reports itself", () => {
  let dir: string;
  let file: string;

  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), "scan-stall-"));
    file = join(dir, "store.json");
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 503 }));
  });
  afterEach(() => {
    rmSync(dir, { recursive: true, force: true });
    vi.unstubAllGlobals();
  });

  // The shape of a real stall now: the chain keeps producing blocks and the relay keeps seeing
  // them, but the scan itself cannot get through. With the settle push gone nothing in the
  // settle path can hold the watermark, so this is what is left — a spoke whose logs cannot be
  // read while its head moves on.
  function headRisesButLogsFail(headRef: { n: number }) {
    return {
      async getBlock(req: { blockHashOrBlockNumber: string | number }) {
        if (req.blockHashOrBlockNumber === 0) return { block: { number: 0, hash: "0xg" } };
        headRef.n += 10;
        return { block: { number: headRef.n } };
      },
      async getPastLogs() {
        throw new Error("besu: connection refused");
      },
      async shutdown() {},
    };
  }

  it("logs an error naming the spoke and how far behind it is", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    await store.setWatermark("spoke-a", 100); // scanned to 100 and then stopped

    const spokes = [
      { id: "spoke-a", besuRpc: "http://a", besuWs: "ws://a", htlcAddress: "0xaaaa", internalApiUrl: "http://a:1", grpcEndpoint: "a:1" },
    ];
    const headRef = { n: 200 };
    const relay = new HtlcRelay(spokes, PROTO, 5, "secret", store as any,
      new Map<string, any>([["spoke-a", headRisesButLogsFail(headRef)]]));

    const errors: string[] = [];
    (relay as any).log = { info: () => {}, warn: () => {}, error: (m: string) => errors.push(m) };
    // Last advance was an hour ago. The threshold is time-based on purpose: distance alone
    // cannot tell a healthy catch-up from a stop.
    (relay as any).watermarkAdvancedAt.set("spoke-a", Date.now() - 3_600_000);

    const controller = new AbortController();
    (relay as any).signal = controller.signal;
    void (relay as any).pollSpoke((relay as any).spokes[0], controller.signal);

    const deadline = Date.now() + 5_000;
    while (Date.now() < deadline && !errors.some(e => /not advanced/i.test(e))) {
      await new Promise(r => setTimeout(r, 10));
    }
    // Keep polling well past the first report — at a 5ms interval that is ~40 more cycles, each
    // of which would re-log if the once-per-stall guard were not there. Aborting on the first
    // error instead would make the "said once" assertion pass against a relay that says it
    // every cycle, which is the noise that hid the original outage.
    await new Promise(r => setTimeout(r, 200));
    controller.abort();

    const joined = errors.join("\n");
    expect(joined).toMatch(/spoke-a/);
    expect(joined).toMatch(/not advanced/i);
    expect(joined).toMatch(/blocks behind/);
    // Said once, not once per poll — the original was hidden by exactly this kind of noise.
    expect(errors.filter(e => /not advanced/i.test(e))).toHaveLength(1);
  }, 15_000);

  // A spoke that has never been scanned is not stalled; reporting it would cry wolf on every
  // fresh bring-up.
  it("does not report a spoke that has never scanned", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();

    const spokes = [
      { id: "spoke-a", besuRpc: "http://a", besuWs: "ws://a", htlcAddress: "0xaaaa", internalApiUrl: "http://a:1", grpcEndpoint: "a:1" },
    ];
    const relay = new HtlcRelay(spokes, PROTO, 5, "secret", store as any,
      new Map<string, any>([["spoke-a", headRisesButLogsFail({ n: 200 })]]));

    const errors: string[] = [];
    (relay as any).log = { info: () => {}, warn: () => {}, error: (m: string) => errors.push(m) };

    const controller = new AbortController();
    (relay as any).signal = controller.signal;
    void (relay as any).pollSpoke((relay as any).spokes[0], controller.signal);
    await new Promise(r => setTimeout(r, 200));
    controller.abort();

    expect(errors.filter(e => /not advanced/i.test(e))).toHaveLength(0);
  }, 15_000);
});

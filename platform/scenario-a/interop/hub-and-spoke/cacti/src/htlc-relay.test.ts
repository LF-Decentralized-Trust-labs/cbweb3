// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { writeFileSync, mkdtempSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { ethers } from "ethers";
import { loadSpokesConfig, buildLegacyShim } from "./spokes-config";
import { HtlcRelay, LockEvent } from "./htlc-relay";
import { RelayStore } from "./relay-store";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeMockRelayStore() {
  return {
    getDueRetries: vi.fn().mockResolvedValue([]),
    hasDelivered: vi.fn().mockResolvedValue(false),
    markDelivered: vi.fn().mockResolvedValue(undefined),
    scheduleRetry: vi.fn().mockResolvedValue(undefined),
    getRetryStats: vi.fn().mockReturnValue({ pending: 0, maxLagMs: 0 }),
    init: vi.fn().mockResolvedValue(undefined),
    appendEvent: vi.fn().mockResolvedValue(1),
    getEventsSince: vi.fn().mockReturnValue([]),
  };
}

const VALID_YAML = `
spokes:
  - id: spoke-a
    besuRpc: "http://host:8645"
    besuWs: "ws://host:8655"
    htlcAddress: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    internalApiUrl: "http://host:18080"
    grpcEndpoint: "host:19094"
  - id: spoke-b
    besuRpc: "http://host:8745"
    besuWs: "ws://host:8755"
    htlcAddress: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    internalApiUrl: "http://host:28080"
    grpcEndpoint: "host:29094"
`;

// ---------------------------------------------------------------------------
// T005 — loadSpokesConfig (YAML path)
// ---------------------------------------------------------------------------

describe("loadSpokesConfig — YAML", () => {
  let tmpDir: string;
  const savedEnv: Record<string, string | undefined> = {};

  beforeEach(() => {
    tmpDir = mkdtempSync(join(tmpdir(), "cacti-test-"));
    savedEnv["CACTI_SPOKES_CONFIG"] = process.env["CACTI_SPOKES_CONFIG"];
    savedEnv["SPOKE_A_BESU_RPC"] = process.env["SPOKE_A_BESU_RPC"];
    savedEnv["SPOKE_B_BESU_RPC"] = process.env["SPOKE_B_BESU_RPC"];
    // Remove legacy shim vars so YAML path is taken
    delete process.env["SPOKE_A_BESU_RPC"];
    delete process.env["SPOKE_B_BESU_RPC"];
  });

  afterEach(() => {
    rmSync(tmpDir, { recursive: true, force: true });
    for (const [k, v] of Object.entries(savedEnv)) {
      if (v === undefined) {
        delete process.env[k];
      } else {
        process.env[k] = v;
      }
    }
  });

  it("(a) returns two SpokeConfig entries from a valid YAML", () => {
    const yamlPath = join(tmpDir, "spokes.yaml");
    writeFileSync(yamlPath, VALID_YAML);
    process.env["CACTI_SPOKES_CONFIG"] = yamlPath;

    const spokes = loadSpokesConfig(console);

    expect(spokes).toHaveLength(2);
    expect(spokes[0]!.id).toBe("spoke-a");
    expect(spokes[0]!.grpcEndpoint).toBe("host:19094");
    expect(spokes[1]!.id).toBe("spoke-b");
    expect(spokes[1]!.grpcEndpoint).toBe("host:29094");
  });

  it("(b) throws Fatal when a required field is missing", () => {
    const yaml = `
spokes:
  - id: spoke-a
    besuWs: "ws://host:8655"
    htlcAddress: "0xaaaa"
    internalApiUrl: "http://host:18080"
    grpcEndpoint: "host:19094"
`;
    const yamlPath = join(tmpDir, "missing-field.yaml");
    writeFileSync(yamlPath, yaml);
    process.env["CACTI_SPOKES_CONFIG"] = yamlPath;

    expect(() => loadSpokesConfig(console)).toThrow("Fatal: spoke[0].besuRpc is required");
  });

  it("(c) throws Fatal when spokes list is empty", () => {
    const yaml = `spokes: []`;
    const yamlPath = join(tmpDir, "empty.yaml");
    writeFileSync(yamlPath, yaml);
    process.env["CACTI_SPOKES_CONFIG"] = yamlPath;

    expect(() => loadSpokesConfig(console)).toThrow("Fatal: at least one spoke must be configured");
  });

  it("(d) throws Fatal on duplicate spoke id", () => {
    const yaml = `
spokes:
  - id: spoke-a
    besuRpc: "http://host:8645"
    besuWs: "ws://host:8655"
    htlcAddress: "0xaaaa"
    internalApiUrl: "http://host:18080"
    grpcEndpoint: "host:19094"
  - id: spoke-a
    besuRpc: "http://host:8745"
    besuWs: "ws://host:8755"
    htlcAddress: "0xbbbb"
    internalApiUrl: "http://host:28080"
    grpcEndpoint: "host:29094"
`;
    const yamlPath = join(tmpDir, "dup.yaml");
    writeFileSync(yamlPath, yaml);
    process.env["CACTI_SPOKES_CONFIG"] = yamlPath;

    expect(() => loadSpokesConfig(console)).toThrow('Fatal: duplicate spoke id "spoke-a"');
  });

  it("(e) throws Fatal when file does not exist", () => {
    const missing = join(tmpDir, "does-not-exist.yaml");
    process.env["CACTI_SPOKES_CONFIG"] = missing;

    expect(() => loadSpokesConfig(console)).toThrow(`Fatal: cannot read spokes config: ${missing}`);
  });
});

// ---------------------------------------------------------------------------
// T006 — buildLegacyShim
// ---------------------------------------------------------------------------

describe("buildLegacyShim", () => {
  const savedEnv: Record<string, string | undefined> = {};
  const LEGACY_VARS = [
    "SPOKE_A_BESU_RPC", "SPOKE_A_BESU_WS", "SPOKE_A_HTLC_ADDRESS",
    "SPOKE_A_INTERNAL_API", "SPOKE_A_PAYMENT_GRPC",
    "SPOKE_B_BESU_RPC", "SPOKE_B_BESU_WS", "SPOKE_B_HTLC_ADDRESS",
    "SPOKE_B_INTERNAL_API", "SPOKE_B_PAYMENT_GRPC",
  ];

  beforeEach(() => {
    for (const k of LEGACY_VARS) {
      savedEnv[k] = process.env[k];
      delete process.env[k];
    }
  });

  afterEach(() => {
    for (const [k, v] of Object.entries(savedEnv)) {
      if (v === undefined) {
        delete process.env[k];
      } else {
        process.env[k] = v;
      }
    }
  });

  function setLegacyEnv(): void {
    process.env["SPOKE_A_BESU_RPC"] = "http://host:8645";
    process.env["SPOKE_A_BESU_WS"] = "ws://host:8655";
    process.env["SPOKE_A_HTLC_ADDRESS"] = "0xaaaa";
    process.env["SPOKE_A_INTERNAL_API"] = "http://host:18080";
    process.env["SPOKE_A_PAYMENT_GRPC"] = "host:19094";
    process.env["SPOKE_B_BESU_RPC"] = "http://host:8745";
    process.env["SPOKE_B_BESU_WS"] = "ws://host:8755";
    process.env["SPOKE_B_HTLC_ADDRESS"] = "0xbbbb";
    process.env["SPOKE_B_INTERNAL_API"] = "http://host:28080";
    process.env["SPOKE_B_PAYMENT_GRPC"] = "host:29094";
  }

  it("(a) returns spoke-a and spoke-b with own grpcEndpoints", () => {
    setLegacyEnv();
    const mockLog = { warn: vi.fn() };

    const spokes = buildLegacyShim(mockLog);

    expect(spokes).toHaveLength(2);
    const a = spokes.find((s) => s.id === "spoke-a");
    const b = spokes.find((s) => s.id === "spoke-b");
    expect(a).toBeDefined();
    expect(b).toBeDefined();
    // Each spoke stores its OWN grpcEndpoint (not the counterpart's)
    expect(a!.grpcEndpoint).toBe("host:19094"); // SPOKE_A_PAYMENT_GRPC
    expect(b!.grpcEndpoint).toBe("host:29094"); // SPOKE_B_PAYMENT_GRPC
  });

  it("(b) throws Fatal when SPOKE_A_BESU_RPC is missing", () => {
    setLegacyEnv();
    delete process.env["SPOKE_A_BESU_RPC"];
    const mockLog = { warn: vi.fn() };

    expect(() => buildLegacyShim(mockLog)).toThrow("Fatal: SPOKE_A_BESU_RPC is required");
  });

  it("(c) emits a deprecation warning mentioning CACTI_SPOKES_CONFIG", () => {
    setLegacyEnv();
    const mockLog = { warn: vi.fn() };

    buildLegacyShim(mockLog);

    expect(mockLog.warn).toHaveBeenCalledWith(
      expect.stringContaining("CACTI_SPOKES_CONFIG"),
    );
  });
});

// ---------------------------------------------------------------------------
// T013 — resolveCounterpart (renamed from resolveCounterpartContractId)
// ---------------------------------------------------------------------------

describe("resolveCounterpart", () => {
  function makeLockEvent(spoke: string, contractId: string, hashLock: string): LockEvent {
    return {
      spoke,
      contractId,
      sender: "0x1",
      receiver: "0x2",
      hashLock,
      timeLock: 9999999999,
      zetoLockRef: "00",
      blockNumber: 1,
      txHash: "0xabc",
      timestamp: Date.now(),
    };
  }

  function makeRelay(): HtlcRelay {
    const spokes = [
      { id: "spoke-a", besuRpc: "http://a:8645", besuWs: "ws://a:8655", htlcAddress: "0xaaaa", internalApiUrl: "http://a:18080", grpcEndpoint: "a:19094" },
      { id: "spoke-b", besuRpc: "http://b:8745", besuWs: "ws://b:8755", htlcAddress: "0xbbbb", internalApiUrl: "http://b:28080", grpcEndpoint: "b:29094" },
    ];
    return new HtlcRelay(
      spokes,
      "/fake/proto",
      3000,
      "secret",
      makeMockRelayStore() as any,
      new Map(),
    );
  }

  it("(a) returns {destSpokeId, contractId} when counterpart lock exists", () => {
    const relay = makeRelay();
    const hashLock = "deadbeef";
    const contractIdA = "contractAAA";
    const contractIdB = "contractBBB";

    (relay as any).lockEvents.push(makeLockEvent("spoke-a", contractIdA, hashLock));
    (relay as any).lockEvents.push(makeLockEvent("spoke-b", contractIdB, hashLock));

    const result = (relay as any).resolveCounterpart("spoke-a", contractIdA);

    expect(result).toBeDefined();
    expect(result.destSpokeId).toBe("spoke-b");
    expect(result.contractId).toBe(contractIdB);
  });

  it("(b) returns undefined and logs warning when no counterpart lock found", () => {
    const relay = makeRelay();
    const mockLog = { info: vi.fn(), warn: vi.fn(), error: vi.fn() };
    (relay as any).log = mockLog;
    (relay as any).lockEvents.push(makeLockEvent("spoke-a", "contractAAA", "hashA"));

    const result = (relay as any).resolveCounterpart("spoke-a", "contractAAA");

    expect(result).toBeUndefined();
    expect(mockLog.warn).toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// T014 — settlement routing by dest_spoke_id
// ---------------------------------------------------------------------------

describe("settlement routing", () => {
  function makeRelay(spokeIds = ["spoke-a", "spoke-b"]): HtlcRelay {
    const spokes = spokeIds.map((id, i) => ({
      id,
      besuRpc: `http://host:${8645 + i * 100}`,
      besuWs: `ws://host:${8655 + i * 100}`,
      htlcAddress: "0xaaaa",
      internalApiUrl: `http://host:${18080 + i * 1000}`,
      grpcEndpoint: `host:${19094 + i * 10000}`,
    }));
    return new HtlcRelay(
      spokes,
      "/fake/proto",
      3000,
      "secret",
      makeMockRelayStore() as any,
      new Map(),
    );
  }

  it("(a) processDueRetriesForSpoke routes to the correct gRPC client by destSpokeId", async () => {
    const relay = makeRelay();
    const mockClientB = {
      close: vi.fn(),
      AcceptFXAgreement: vi.fn((_req: unknown, _meta: unknown, _opts: unknown, cb: (err: null, resp: { tx_hash: string }) => void) =>
        cb(null, { tx_hash: "0xdone" }),
      ),
    };
    (relay as any).grpcClients.set("spoke-a", { close: vi.fn() });
    (relay as any).grpcClients.set("spoke-b", mockClientB);

    const mockStore = makeMockRelayStore();
    mockStore.getDueRetries.mockResolvedValue([
      {
        action: "accept",
        spokeName: "spoke-a",
        tradeId: "trade-1",
        payload: { destSpokeId: "spoke-b" },
        lastError: "prev",
      },
    ]);
    (relay as any).relayStore = mockStore;

    await (relay as any).processDueRetriesForSpoke("spoke-a");

    expect(mockClientB.AcceptFXAgreement).toHaveBeenCalled();
  });

  it("(b) unknown dest_spoke_id does not crash the relay and logs an error", async () => {
    const relay = makeRelay(["spoke-a", "spoke-b"]);
    const mockLog = { info: vi.fn(), warn: vi.fn(), error: vi.fn() };
    (relay as any).log = mockLog;

    // Simulate a retry item referencing an unknown spoke
    const mockStore = makeMockRelayStore();
    mockStore.getDueRetries.mockResolvedValue([
      { action: "accept", spokeName: "spoke-a", tradeId: "trade-1", payload: { destSpokeId: "spoke-c" }, lastError: "prev" },
    ]);
    (relay as any).relayStore = mockStore;

    // Before T020, processDueRetriesForSpoke still takes a client param.
    // After T020 it reads destSpokeId from payload and logs error for unknown spoke.
    // This call will fail before T020 because signature is different.
    await (relay as any).processDueRetriesForSpoke("spoke-a");

    expect(mockLog.error).toHaveBeenCalledWith(
      expect.stringContaining("spoke-c"),
    );
  });
});

// ---------------------------------------------------------------------------
// Phase 4 — dynamic spoke lifecycle (registry-driven watchers)
// ---------------------------------------------------------------------------

describe("dynamic spoke lifecycle", () => {
  const spokeA = {
    id: "spoke-a", besuRpc: "http://a:8645", besuWs: "ws://a:8655",
    htlcAddress: "0xaaaa", internalApiUrl: "http://a:18080", grpcEndpoint: "a:19094",
  };

  function relayWithFactory(factory?: (s: any) => Promise<any>) {
    return new HtlcRelay(
      [], "/fake/proto", 3000, "secret",
      makeMockRelayStore() as any,
      new Map(),
      factory as any,
    );
  }

  it("(a) addSpoke before start() does not begin watching", async () => {
    const factory = vi.fn();
    const relay = relayWithFactory(factory as any);
    (relay as any).log = { info: vi.fn(), warn: vi.fn(), error: vi.fn() };
    await relay.addSpoke(spokeA);
    expect((relay as any).watching.has("spoke-a")).toBe(false);
    expect(factory).not.toHaveBeenCalled();
  });

  it("(b) addSpoke with no connector and no factory does not watch", async () => {
    const relay = relayWithFactory(undefined);
    const mockLog = { info: vi.fn(), warn: vi.fn(), error: vi.fn() };
    (relay as any).log = mockLog;
    relay.start(new AbortController().signal);
    await relay.addSpoke(spokeA);
    expect((relay as any).watching.has("spoke-a")).toBe(false);
    expect(mockLog.error).toHaveBeenCalledWith(expect.stringContaining("no Cacti connector"));
  });

  it("(c) addSpoke is idempotent — a spoke already watched is not re-added", async () => {
    const factory = vi.fn();
    const relay = relayWithFactory(factory as any);
    (relay as any).log = { info: vi.fn(), warn: vi.fn(), error: vi.fn() };
    relay.start(new AbortController().signal);
    (relay as any).watching.add("spoke-a"); // simulate an active watcher
    await relay.addSpoke(spokeA);
    expect(factory).not.toHaveBeenCalled(); // returned early; no connector created
    expect((relay as any).grpcClients.has("spoke-a")).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// R2-H-11 — HTLC settle path: persisted dedup + restart produces no duplicate settle
// ---------------------------------------------------------------------------

describe("HTLC settle persistence (R2-H-11)", () => {
  const CLAIMED_TOPIC = ethers.id("LogHTLCClaimed(bytes32,bytes32)");
  const CONTRACT_A = "0x" + "aa".repeat(32); // source contractId (spoke-a)
  const CONTRACT_B = "bb".repeat(32);        // counterpart contractId (spoke-b), stripped form
  const HASHLOCK = "deadbeef";
  const TX = "0xtx";
  const EVENT_BLOCK = 10;
  // Mirrors MAX_SETTLE_ATTEMPTS in htlc-relay.ts (not exported); asserts we stayed below the cap.
  const MAX_ATTEMPTS_IN_TEST = 20;

  let dir: string;
  let file: string;

  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), "htlc-settle-test-"));
    file = join(dir, "store.json");
    // Silence FX REST polling: pollSpoke calls fetch(internalApiUrl); return a benign non-OK.
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 503 }));
  });

  afterEach(() => {
    rmSync(dir, { recursive: true, force: true });
    vi.unstubAllGlobals();
  });

  // A decodable LogHTLCClaimed(contractId, secret) log at EVENT_BLOCK.
  function claimedLog() {
    const data = ethers.AbiCoder.defaultAbiCoder().encode(["bytes32"], ["0x" + "11".repeat(32)]);
    return {
      topics: [CLAIMED_TOPIC, CONTRACT_A],
      data,
      blockNumber: EVENT_BLOCK,
      transactionHash: TX,
      logIndex: 0,
    };
  }

  function fakeConnector() {
    return {
      async getBlock(req: { blockHashOrBlockNumber: string | number }) {
        if (req.blockHashOrBlockNumber === 0) {
          return { block: { number: 0, hash: "0xgenesis" } };
        }
        return { block: { number: EVENT_BLOCK } };
      },
      async getPastLogs(args: { topics: unknown }) {
        const isClaimed = JSON.stringify(args.topics).includes(CLAIMED_TOPIC);
        return { logs: isClaimed ? [claimedLog()] : [] };
      },
      async shutdown() {},
    };
  }

  // Build a relay wired with the real store, a fake connector for spoke-a, a mock gRPC client
  // for spoke-b, and the two counterpart lock events pre-seeded (as an already-running relay
  // would have observed). Returns the relay and the SettleHTLC call counter.
  async function makeWiredRelay(store: RelayStore, alwaysFail = false) {
    const spokes = [
      { id: "spoke-a", besuRpc: "http://a", besuWs: "ws://a", htlcAddress: "0xaaaa", internalApiUrl: "http://a:18080", grpcEndpoint: "a:1" },
      { id: "spoke-b", besuRpc: "http://b", besuWs: "ws://b", htlcAddress: "0xbbbb", internalApiUrl: "http://b:28080", grpcEndpoint: "b:1" },
    ];
    const connectors = new Map<string, any>([["spoke-a", fakeConnector()]]);
    const relay = new HtlcRelay(spokes, "/fake/proto", 5, "secret", store as any, connectors);
    (relay as any).log = { info: () => {}, warn: () => {}, error: () => {} };
    (relay as any).signal = new AbortController().signal;

    const settleCalls = { n: 0 };
    const mockClientB = {
      close: vi.fn(),
      SettleHTLC: (
        _req: unknown,
        _m: unknown,
        _o: unknown,
        cb: (e: unknown, r?: { htlc_tx_hash: string; zeto_tx_hash: string }) => void,
      ) => {
        settleCalls.n++;
        if (alwaysFail) {
          // What the LNET relay actually got, for ever: the destination spoke's registered
          // endpoint is its CENTRAL BANK's orchestrator, and the contract lives on a
          // commercial bank's. The error is permanent — retrying never changes it.
          cb({ code: 5, message: `5 NOT_FOUND: HTLC "${CONTRACT_B}" not found` });
          return;
        }
        cb(null, { htlc_tx_hash: "0xh", zeto_tx_hash: "0xz" });
      },
    };
    (relay as any).grpcClients.set("spoke-b", mockClientB);

    // Pre-seed the counterpart lock pair so resolveCounterpart succeeds.
    const lock = (spoke: string, contractId: string): LockEvent => ({
      spoke, contractId, sender: "0x1", receiver: "0x2", hashLock: HASHLOCK,
      timeLock: 9999999999, zetoLockRef: "00", blockNumber: 1, txHash: "0xl", timestamp: Date.now(),
    });
    (relay as any).lockEvents.push(lock("spoke-a", "aa".repeat(32)));
    (relay as any).lockEvents.push(lock("spoke-b", CONTRACT_B));

    return { relay, settleCalls };
  }

  async function waitFor(cond: () => boolean, timeoutMs = 1_500): Promise<boolean> {
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
      if (cond()) return true;
      await new Promise(r => setTimeout(r, 5));
    }
    return cond();
  }

  it("settles once and persists the dedup keys + watermark", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    const { relay, settleCalls } = await makeWiredRelay(store);

    const controller = new AbortController();
    (relay as any).signal = controller.signal;
    void (relay as any).pollSpoke((relay as any).spokes[0], controller.signal);

    await waitFor(() => settleCalls.n >= 1);
    controller.abort();
    await new Promise(r => setTimeout(r, 20));

    expect(settleCalls.n).toBe(1);
    // Both guards persisted, and the watermark advanced to the processed block.
    const reloaded = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await reloaded.init();
    expect(reloaded.hasDelivered(`htlc-evt:spoke-a:${TX}:0`)).toBe(true);
    expect(reloaded.hasDelivered(`htlc-settled:spoke-b:${CONTRACT_B}`)).toBe(true);
    expect(reloaded.getWatermark("spoke-a")).toBe(EVENT_BLOCK);
  });

  // ── the outage this file did not cover ────────────────────────────────────────
  //
  // A settlement that can never succeed used to pin the block watermark at its own block,
  // for ever. The hold is on the BLOCK, so it froze every unrelated event after it on that
  // spoke — locks included — and nothing said so. On LNET that was 167,360 identical
  // failures and three days in which no PvP could pair, presenting as "it just stopped".
  it("a permanently failing settlement releases the watermark instead of freezing the spoke", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    const { relay, settleCalls } = await makeWiredRelay(store, true);

    const errors: string[] = [];
    (relay as any).log = { info: () => {}, warn: () => {}, error: (m: string) => errors.push(m) };

    const controller = new AbortController();
    (relay as any).signal = controller.signal;
    void (relay as any).pollSpoke((relay as any).spokes[0], controller.signal);

    // Give up is bounded, so the watermark must reach the event's block on its own.
    const advanced = await waitFor(() => store.getWatermark("spoke-a") === EVENT_BLOCK, 8_000);
    controller.abort();
    await new Promise(r => setTimeout(r, 20));

    expect(advanced).toBe(true);
    expect(store.getWatermark("spoke-a")).toBe(EVENT_BLOCK);

    // It gave up rather than looping without end, and said so where an operator will see it.
    expect(settleCalls.n).toBeGreaterThanOrEqual(1);
    expect(settleCalls.n).toBeLessThanOrEqual(40);
    expect(errors.join("\n")).toMatch(/failed \d+ times/);
    expect(errors.join("\n")).toMatch(/NOT settled by the relay/);
  }, 15_000);

  // The bound must not become "skip on the first hiccup": a destination that is merely
  // restarting has to be retried, which is the reason the hold exists at all.
  it("holds the watermark while a settlement failure is still plausibly transient", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    const { relay, settleCalls } = await makeWiredRelay(store, true);
    (relay as any).log = { info: () => {}, warn: () => {}, error: () => {} };

    // Fail once, then never answer. Parking the second attempt pins the relay in the state
    // under test — one recorded failure, watermark held — no matter how slow the host is.
    // Racing an abort against the give-up instead would make this test a coin flip on a
    // loaded CI box: the cap is 20 attempts and the poll interval here is 5ms, so a ~150ms
    // stall between "first failure observed" and "abort" reaches the cap and flips every
    // assertion below.
    (relay as any).grpcClients.set("spoke-b", {
      close: vi.fn(),
      SettleHTLC: (_r: unknown, _m: unknown, _o: unknown, cb: (e: unknown) => void) => {
        settleCalls.n++;
        if (settleCalls.n === 1) cb({ code: 5, message: "5 NOT_FOUND" });
        // else: never call back — the cycle parks here.
      },
    });

    const controller = new AbortController();
    (relay as any).signal = controller.signal;
    void (relay as any).pollSpoke((relay as any).spokes[0], controller.signal);

    await waitFor(() => store.settleFailureCount(`htlc-evt:spoke-a:${TX}:0`) >= 1, 5_000);
    controller.abort();

    expect(settleCalls.n).toBeLessThan(MAX_ATTEMPTS_IN_TEST);
    expect(store.settleFailureCount(`htlc-evt:spoke-a:${TX}:0`)).toBe(1);
    expect(store.getWatermark("spoke-a")).toBeLessThan(EVENT_BLOCK);
    expect(store.hasDelivered(`htlc-evt:spoke-a:${TX}:0`)).toBe(false);
  });

  // ── review findings on the bound itself ──────────────────────────────────────
  //
  // (2) Giving up must forget the failure history. The counter is persisted, so leaving it at
  // the cap turns a later forced retry — the operator's move once the routing is fixed — into
  // an immediate give-up on the very first attempt.
  it("clears the failure counter when it gives up, so a forced retry still gets its retries", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    const { relay } = await makeWiredRelay(store, true);
    (relay as any).log = { info: () => {}, warn: () => {}, error: () => {} };

    const controller = new AbortController();
    (relay as any).signal = controller.signal;
    void (relay as any).pollSpoke((relay as any).spokes[0], controller.signal);

    const evtKey = `htlc-evt:spoke-a:${TX}:0`;
    await waitFor(() => store.hasDelivered(evtKey), 8_000);
    controller.abort();
    await new Promise(r => setTimeout(r, 20));

    expect(store.hasDelivered(evtKey)).toBe(true);
    expect(store.settleFailureCount(evtKey)).toBe(0);

    // And it must not come back from disk either.
    const reloaded = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await reloaded.init();
    expect(reloaded.settleFailureCount(evtKey)).toBe(0);
  }, 15_000);

  // (3) Counting a failure writes the store to disk, and that write can fail (ENOSPC, read-only
  // volume). It must not cost us the watermark hold: the settlement did NOT deliver, so the block
  // has to be retried. Before the bound existed this branch was a pure in-memory assignment and
  // could not throw — the counter is what introduced the risk, so the counter must carry it.
  it("holds the watermark when persisting the failure counter throws", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    const { relay, settleCalls } = await makeWiredRelay(store, true);

    const logged: string[] = [];
    (relay as any).log = {
      info: () => {}, warn: (m: string) => logged.push(m), error: (m: string) => logged.push(m),
    };

    // The disk is full exactly when the relay tries to record the failure.
    vi.spyOn(store, "recordSettleFailure").mockRejectedValue(
      Object.assign(new Error("ENOSPC: no space left on device"), { code: "ENOSPC" }),
    );

    const controller = new AbortController();
    (relay as any).signal = controller.signal;
    void (relay as any).pollSpoke((relay as any).spokes[0], controller.signal);

    await waitFor(() => settleCalls.n >= 2, 5_000);
    controller.abort();
    await new Promise(r => setTimeout(r, 20));

    // The claim is unsettled and unrecorded, so the block must be retried, not skipped.
    expect(store.getWatermark("spoke-a")).toBeLessThan(EVENT_BLOCK);
    expect(store.hasDelivered(`htlc-evt:spoke-a:${TX}:0`)).toBe(false);
    // And it must not be reported as a decode problem, which sends an operator to the wrong place.
    expect(logged.join("\n")).not.toMatch(/failed to decode/);
  }, 15_000);

  it("does NOT re-settle after a restart when the claim was already delivered", async () => {
    // Simulate a crash AFTER the settle was forwarded (dedup key persisted) but BEFORE the
    // watermark advanced past the event's block — so the restart re-scans the same block.
    const seed = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await seed.init();
    await seed.setWatermark("spoke-a", EVENT_BLOCK - 1); // watermark still behind the event
    await seed.markDelivered(`htlc-evt:spoke-a:${TX}:0`); // but the event was already delivered

    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    const { relay, settleCalls } = await makeWiredRelay(store);

    const controller = new AbortController();
    (relay as any).signal = controller.signal;
    void (relay as any).pollSpoke((relay as any).spokes[0], controller.signal);

    // Let several poll cycles run; the event is re-scanned but must be skipped.
    await new Promise(r => setTimeout(r, 80));
    controller.abort();
    await new Promise(r => setTimeout(r, 20));

    expect(settleCalls.n).toBe(0);
  });
});

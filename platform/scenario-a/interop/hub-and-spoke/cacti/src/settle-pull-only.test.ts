// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { mkdtempSync, rmSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { ethers } from "ethers";
import { HtlcRelay, LockEvent } from "./htlc-relay";
import { RelayStore } from "./relay-store";

// The relay does not settle a counterpart leg, and must not try.
//
// Settling an HTLC leg is transferLocked on the owner's own Paladin node, over locked Zeto
// states that are local to that node. No one can do it for the owner. The relay could not even
// address them: it holds one gRPC endpoint per spoke — the founding central bank's, because a
// commercial bank's join never registers — while an inter-bank leg lives on a commercial bank's
// orchestrator. Every push for such a trade answered NOT_FOUND, permanently, and the failure
// held the block watermark, which froze the whole spoke's chain scan and starved the journal
// that is the path that actually works. Measured on LNET: 167,360 identical failures, three
// days with no PvP able to pair, and both legs of the trade SETTLED the whole time — by the
// pull path.
//
// What delivers a settlement is this: the claim, secret included, is appended to the durable
// journal, and each orchestrator polls it and settles its own leg, resolving it by
// sha256(secret) rather than by a contract id it was told. These tests hold that line.
describe("settle is pull-only: the relay journals the claim and pushes nothing", () => {
  const CLAIMED_TOPIC = ethers.id("LogHTLCClaimed(bytes32,bytes32)");
  const CONTRACT_A = "0x" + "aa".repeat(32);
  const SECRET = "11".repeat(32);
  const CLAIM_BLOCK = 10;
  const HEAD = 40;

  let dir: string;
  let file: string;

  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), "pull-only-"));
    file = join(dir, "store.json");
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 503 }));
  });
  afterEach(() => {
    rmSync(dir, { recursive: true, force: true });
    vi.unstubAllGlobals();
  });

  function claimedLog() {
    return {
      topics: [CLAIMED_TOPIC, CONTRACT_A],
      data: ethers.AbiCoder.defaultAbiCoder().encode(["bytes32"], ["0x" + SECRET]),
      blockNumber: CLAIM_BLOCK,
      transactionHash: "0xclaimtx",
      logIndex: 0,
    };
  }

  function fakeConnector() {
    return {
      async getBlock(req: { blockHashOrBlockNumber: string | number }) {
        if (req.blockHashOrBlockNumber === 0) return { block: { number: 0, hash: "0xgenesis" } };
        return { block: { number: HEAD } };
      },
      async getPastLogs(args: { topics: unknown; fromBlock: number; toBlock: number }) {
        const wantClaimed = JSON.stringify(args.topics).includes(CLAIMED_TOPIC);
        if (!wantClaimed) return { logs: [] };
        const l = claimedLog();
        return { logs: l.blockNumber >= args.fromBlock && l.blockNumber <= args.toBlock ? [l] : [] };
      },
      async shutdown() {},
    };
  }

  async function run(store: RelayStore) {
    const spokes = [
      { id: "spoke-a", besuRpc: "http://a", besuWs: "ws://a", htlcAddress: "0xaaaa", internalApiUrl: "http://a:1", grpcEndpoint: "a:1" },
      { id: "spoke-b", besuRpc: "http://b", besuWs: "ws://b", htlcAddress: "0xbbbb", internalApiUrl: "http://b:1", grpcEndpoint: "b:1" },
    ];
    const relay = new HtlcRelay(spokes, "/fake/proto", 5, "secret", store as any,
      new Map<string, any>([["spoke-a", fakeConnector()]]));
    (relay as any).log = { info: () => {}, warn: () => {}, error: () => {} };

    // If anything in the settle path reaches for a gRPC client, this is what it gets.
    const settleCalls = { n: 0 };
    (relay as any).grpcClients.set("spoke-b", {
      close: vi.fn(),
      SettleHTLC: (_r: unknown, _m: unknown, _o: unknown, cb: (e: unknown) => void) => {
        settleCalls.n++;
        cb({ code: 5, message: "5 NOT_FOUND" });
      },
    });

    // Seed the counterpart lock pair. Without it the OLD code bails out of the push before it
    // is made, and these tests would pass against the very behaviour they exist to forbid.
    const HASHLOCK = ethers.sha256("0x" + SECRET).slice(2);
    const lock = (spoke: string, contractId: string): LockEvent => ({
      spoke, contractId, sender: "0x1", receiver: "0x2", hashLock: HASHLOCK,
      timeLock: 9999999999, zetoLockRef: "00", blockNumber: 1, txHash: "0xl", timestamp: Date.now(),
    });
    (relay as any).lockEvents?.push(lock("spoke-a", "aa".repeat(32)));
    (relay as any).lockEvents?.push(lock("spoke-b", "bb".repeat(32)));

    const controller = new AbortController();
    (relay as any).signal = controller.signal;
    void (relay as any).pollSpoke((relay as any).spokes[0], controller.signal);
    return { relay, settleCalls, controller };
  }

  async function waitFor(cond: () => boolean, timeoutMs = 5_000): Promise<boolean> {
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
      if (cond()) return true;
      await new Promise(r => setTimeout(r, 5));
    }
    return cond();
  }

  it("journals the claim with its secret — the only thing a destination leg needs", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    await store.setWatermark("spoke-a", CLAIM_BLOCK - 1);

    const { controller } = await run(store);
    const ok = await waitFor(() => store.getEventsSince("settle", 0).length > 0);
    controller.abort();

    expect(ok).toBe(true);
    const [evt] = store.getEventsSince("settle", 0);
    expect(evt["spoke"]).toBe("spoke-a");
    expect(evt["contractId"]).toBe("aa".repeat(32));
    // The secret is the whole payload: each orchestrator finds its own leg by sha256(secret),
    // so it never needs to be told which contract is its own.
    expect(evt["secret"]).toBe(SECRET);
  });

  it("never calls SettleHTLC on a counterpart", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    await store.setWatermark("spoke-a", CLAIM_BLOCK - 1);

    const { settleCalls, controller } = await run(store);
    await waitFor(() => store.getEventsSince("settle", 0).length > 0);
    // Give the old code every chance to make the call it used to make.
    await new Promise(r => setTimeout(r, 150));
    controller.abort();

    expect(settleCalls.n).toBe(0);
  });

  // The watermark used to be held by a failed push, which is what froze the spoke. With no push
  // there is nothing that can hold it: once the claim is journaled the scan moves on, every time.
  it("advances the watermark past the claim to the chain head", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    await store.setWatermark("spoke-a", CLAIM_BLOCK - 1);

    const { controller } = await run(store);
    const advanced = await waitFor(() => store.getWatermark("spoke-a") === HEAD);
    controller.abort();

    expect(advanced).toBe(true);
  });

  // The guarantee the removed "does NOT re-settle after a restart" test protected: a relay that
  // comes back and re-scans the same block must not deliver the claim twice. It is now the
  // journal's job — appendEvent is idempotent by chain identity — rather than a dedup key
  // guarding a network call, so prove it end to end through the poll loop.
  it("re-scanning the same block after a restart adds no second journal entry", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    await store.setWatermark("spoke-a", CLAIM_BLOCK - 1);

    const first = await run(store);
    await waitFor(() => store.getEventsSince("settle", 0).length > 0);
    first.controller.abort();
    await new Promise(r => setTimeout(r, 20));
    const seqAfterFirst = store.getEventsSince("settle", 0)[0]["seq"];

    // Restart from a watermark that has not yet passed the claim — the crash window.
    const reloaded = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await reloaded.init();
    await reloaded.setWatermark("spoke-a", CLAIM_BLOCK - 1);
    const second = await run(reloaded);
    await waitFor(() => reloaded.getWatermark("spoke-a") === HEAD);
    second.controller.abort();
    await new Promise(r => setTimeout(r, 20));

    const entries = reloaded.getEventsSince("settle", 0);
    expect(entries).toHaveLength(1);
    // Same seq, so a consumer that already processed it does not see it again.
    expect(entries[0]["seq"]).toBe(seqAfterFirst);
  }, 20_000);

  // No settle-failure bookkeeping survives, because nothing can fail any more. A store that
  // still carries the key from an older relay must load, and must not grow a new one.
  it("records no settle-failure state", async () => {
    const store = new RelayStore(file, { info: () => {}, warn: () => {}, error: () => {} });
    await store.init();
    await store.setWatermark("spoke-a", CLAIM_BLOCK - 1);

    const { controller } = await run(store);
    await waitFor(() => store.getWatermark("spoke-a") === HEAD);
    controller.abort();
    await new Promise(r => setTimeout(r, 20));

    const onDisk = JSON.parse(readFileSync(file, "utf8")) as Record<string, unknown>;
    expect(Object.keys((onDisk["settleFailures"] ?? {}) as object)).toHaveLength(0);
  });
});

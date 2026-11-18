// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mkdtempSync, rmSync } from "node:fs";
import { join, resolve } from "node:path";
import { tmpdir } from "node:os";
import { HtlcRelay } from "./htlc-relay";
import { RelayStore } from "./relay-store";

// When the relay rebuilds a spoke's Besu connector, closing the old one is what gives its
// WebSocket slot back to Besu (see closing-besu-connector.test.ts). A close that fails must be
// reported: swallowing it is how the slot leak stayed invisible until Besu refused every client.

const PROTO = resolve(__dirname, "../../../../apis/proto/payment_orchestrator/v1/payment_orchestrator.proto");
const quiet = { info: () => {}, warn: () => {}, error: () => {} };

describe("connector rebuild after failed poll cycles", () => {
  let dir: string;

  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), "connector-reconnect-"));
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 503 }));
  });
  afterEach(() => {
    rmSync(dir, { recursive: true, force: true });
    vi.unstubAllGlobals();
  });

  async function runUntilRebuilt(
    oldShutdown: () => Promise<void>,
    opts: { failBuilds?: number; events?: string[] } = {},
  ) {
    const store = new RelayStore(join(dir, "store.json"), quiet);
    await store.init();

    const warnings: string[] = [];
    const shutdowns = vi.fn(async () => {
      opts.events?.push("close-old");
      await oldShutdown();
    });
    let built = 0;
    let attempts = 0;
    const factory = async () => {
      attempts++;
      opts.events?.push(`build#${attempts}`);
      // Build 1 is the registration; the next failBuilds rebuild attempts are refused.
      if (attempts > 1 && attempts <= 1 + (opts.failBuilds ?? 0)) {
        throw new Error("connection refused: too many WebSocket connections");
      }
      built++;
      if (built === 1) {
        // The connector attached at registration: every poll fails the way a dropped WebSocket does.
        return {
          async getBlock() { throw new Error("connection not open on send()"); },
          async getPastLogs() { return { logs: [] }; },
          shutdown: shutdowns,
        } as any;
      }
      return {
        async getBlock() { return { block: { number: 0, hash: "0xg" } }; },
        async getPastLogs() { return { logs: [] }; },
        async shutdown() {},
      } as any;
    };

    const relay = new HtlcRelay([], PROTO, 5, "secret", store as any, new Map(), factory, {
      info: () => {},
      warn: (m: string) => warnings.push(String(m)),
      error: (m: string) => warnings.push(String(m)),
    });
    const controller = new AbortController();
    relay.start(controller.signal);
    await relay.addSpoke({
      id: "spoke-a", besuRpc: "http://a", besuWs: "ws://a", htlcAddress: "0xaaaa",
      internalApiUrl: "http://a:1", grpcEndpoint: "a:1",
    } as any);

    const deadline = Date.now() + 10_000;
    while (built < 2 && Date.now() < deadline) await new Promise((r) => setTimeout(r, 10));
    await new Promise((r) => setTimeout(r, 50));
    controller.abort();
    return { built, shutdowns, warnings };
  }

  it("closes the connector it replaces", async () => {
    const { built, shutdowns } = await runUntilRebuilt(async () => {});
    expect(built).toBeGreaterThanOrEqual(2);
    expect(shutdowns).toHaveBeenCalledTimes(1);
  }, 15_000);

  it("reports a close that fails instead of discarding it", async () => {
    const { built, warnings } = await runUntilRebuilt(async () => {
      throw new Error("provider gone");
    });
    expect(built).toBeGreaterThanOrEqual(2);
    expect(warnings.some((w) => w.includes("spoke-a") && w.includes("provider gone"))).toBe(true);
  }, 15_000);

  // When Besu is at its connection ceiling, the old connector's slot is the one the new socket
  // needs, so the old connector must be closed before the new one is built.
  it("closes the old connector before building the new one", async () => {
    const events: string[] = [];
    const { built } = await runUntilRebuilt(async () => {}, { events });
    expect(built).toBeGreaterThanOrEqual(2);
    expect(events.slice(0, 3)).toEqual(["build#1", "close-old", "build#2"]);
  }, 15_000);

  // A rebuild that cannot build its replacement has already closed the old connector; the next
  // attempt must not close it again, only retry the build.
  it("closes the old connector once across failed rebuild attempts", async () => {
    const events: string[] = [];
    const { built, shutdowns } = await runUntilRebuilt(async () => {}, { failBuilds: 1, events });
    expect(built).toBeGreaterThanOrEqual(2);
    expect(shutdowns).toHaveBeenCalledTimes(1);
    expect(events.slice(0, 4)).toEqual(["build#1", "close-old", "build#2", "build#3"]);
  }, 20_000);
});

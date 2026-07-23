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
});

// SPDX-License-Identifier: Apache-2.0

import { describe, it, expect, beforeEach, afterEach } from "vitest";
import { mkdtempSync, rmSync, existsSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import {
  SpokeRegistry,
  normalizeSpokeRegistration,
  SpokeRegistrationError,
} from "./spoke-registry";

describe("normalizeSpokeRegistration", () => {
  it("accepts the snake_case shape sent by the Go relay registrar", () => {
    const spoke = normalizeSpokeRegistration({
      spoke_id: "spoke-brl",
      besu_rpc_url: "http://localhost:8645",
      besu_ws_url: "ws://localhost:8655",
      htlc_address: "0xabc",
      grpc_endpoint: "localhost:19094",
    });
    expect(spoke.id).toBe("spoke-brl");
    expect(spoke.besuRpc).toBe("http://localhost:8645");
    expect(spoke.besuWs).toBe("ws://localhost:8655");
    expect(spoke.htlcAddress).toBe("0xabc");
    expect(spoke.grpcEndpoint).toBe("localhost:19094");
    expect(spoke.registeredAt).toBeGreaterThan(0);
  });

  it("accepts a camelCase shape", () => {
    const spoke = normalizeSpokeRegistration({
      id: "spoke-cop",
      besuRpc: "http://localhost:8745",
    });
    expect(spoke.id).toBe("spoke-cop");
    expect(spoke.besuRpc).toBe("http://localhost:8745");
    // Optional fields default to empty string.
    expect(spoke.besuWs).toBe("");
    expect(spoke.internalApiUrl).toBe("");
  });

  it("throws when id is missing", () => {
    expect(() => normalizeSpokeRegistration({ besu_rpc_url: "http://x:8645" })).toThrow(
      SpokeRegistrationError,
    );
  });

  it("throws when besuRpc is missing", () => {
    expect(() => normalizeSpokeRegistration({ spoke_id: "spoke-brl" })).toThrow(
      SpokeRegistrationError,
    );
  });
});

describe("SpokeRegistry", () => {
  let dir: string;
  let filePath: string;

  beforeEach(() => {
    dir = mkdtempSync(join(tmpdir(), "spoke-registry-"));
    filePath = join(dir, "registry.json");
  });

  afterEach(() => {
    rmSync(dir, { recursive: true, force: true });
  });

  it("registers a spoke and answers has/get/list", async () => {
    const reg = new SpokeRegistry(filePath, { info() {}, warn() {}, error() {} });
    await reg.load();

    expect(reg.has("spoke-brl")).toBe(false);

    const spoke = normalizeSpokeRegistration({ spoke_id: "spoke-brl", besu_rpc_url: "http://x:8645" });
    await reg.register(spoke);

    expect(reg.has("spoke-brl")).toBe(true);
    expect(reg.get("spoke-brl")?.besuRpc).toBe("http://x:8645");
    expect(reg.list()).toHaveLength(1);
    expect(existsSync(filePath)).toBe(true);
  });

  it("upserts by id (idempotent re-registration)", async () => {
    const reg = new SpokeRegistry(filePath, { info() {}, warn() {}, error() {} });
    await reg.load();
    await reg.register(normalizeSpokeRegistration({ spoke_id: "spoke-brl", besu_rpc_url: "http://x:8645" }));
    await reg.register(normalizeSpokeRegistration({ spoke_id: "spoke-brl", besu_rpc_url: "http://y:8646" }));
    expect(reg.list()).toHaveLength(1);
    expect(reg.get("spoke-brl")?.besuRpc).toBe("http://y:8646");
  });

  it("persists across instances (survives restart)", async () => {
    const reg1 = new SpokeRegistry(filePath, { info() {}, warn() {}, error() {} });
    await reg1.load();
    await reg1.register(normalizeSpokeRegistration({ spoke_id: "spoke-cop", besu_rpc_url: "http://z:8745" }));

    const reg2 = new SpokeRegistry(filePath, { info() {}, warn() {}, error() {} });
    await reg2.load();
    expect(reg2.has("spoke-cop")).toBe(true);
    expect(reg2.get("spoke-cop")?.besuRpc).toBe("http://z:8745");
  });

  it("load tolerates a missing file (starts empty)", async () => {
    const reg = new SpokeRegistry(join(dir, "does-not-exist.json"), { info() {}, warn() {}, error() {} });
    await reg.load();
    expect(reg.list()).toHaveLength(0);
  });
});

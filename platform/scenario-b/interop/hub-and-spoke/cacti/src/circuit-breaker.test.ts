// SPDX-License-Identifier: Apache-2.0
import { test } from "node:test";
import assert from "node:assert/strict";
import { checkNotPaused, CircuitBreakerLogger, IsPausedReader } from "./circuit-breaker";

const AMM = "0x1111111111111111111111111111111111111111";

// Capturing logger: records each JSON warn line so tests can assert the refusal reason.
function capturingLogger(): { log: CircuitBreakerLogger; lines: Record<string, unknown>[] } {
  const lines: Record<string, unknown>[] = [];
  return {
    lines,
    log: { warn: (m: string) => lines.push(JSON.parse(m) as Record<string, unknown>) },
  };
}

test("paused AMM → not safe to forward (recusa), logged reason=paused", async () => {
  const read: IsPausedReader = async () => true;
  const { log, lines } = capturingLogger();
  assert.equal(await checkNotPaused(AMM, read, log), false);
  assert.equal(lines.length, 1);
  assert.equal(lines[0]["event"], "circuit_breaker_refused");
  assert.equal(lines[0]["reason"], "paused");
});

test("active AMM → safe to forward, no refusal logged", async () => {
  const read: IsPausedReader = async () => false;
  const { log, lines } = capturingLogger();
  assert.equal(await checkNotPaused(AMM, read, log), true);
  assert.equal(lines.length, 0);
});

test("read error/unavailable → fail-safe (recusa), logged reason=read_error with message", async () => {
  const read: IsPausedReader = async () => { throw new Error("connection refused"); };
  const { log, lines } = capturingLogger();
  assert.equal(await checkNotPaused(AMM, read, log), false);
  assert.equal(lines.length, 1);
  assert.equal(lines[0]["reason"], "read_error");
  // The underlying error must be surfaced, NOT reported as "paused" — this is the silent
  // fail-safe that masked the wrong HUB_BESU_RPC port.
  assert.match(String(lines[0]["error"]), /connection refused/);
});

test("missing/invalid amm_address → fail-safe (recusa), logged reason=missing_address", async () => {
  const read: IsPausedReader = async () => false; // would be "active", but address invalid
  for (const bad of [undefined, "not-an-address", "0x1234"]) {
    const { log, lines } = capturingLogger();
    assert.equal(await checkNotPaused(bad, read, log), false);
    assert.equal(lines.length, 1);
    assert.equal(lines[0]["reason"], "missing_address");
  }
});

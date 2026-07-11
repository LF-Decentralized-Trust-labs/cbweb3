// SPDX-License-Identifier: Apache-2.0
import { test } from "node:test";
import assert from "node:assert/strict";
import { checkNotPaused, IsPausedReader } from "./circuit-breaker";

const AMM = "0x1111111111111111111111111111111111111111";

test("paused AMM → not safe to forward (recusa)", async () => {
  const read: IsPausedReader = async () => true;
  assert.equal(await checkNotPaused(AMM, read), false);
});

test("active AMM → safe to forward", async () => {
  const read: IsPausedReader = async () => false;
  assert.equal(await checkNotPaused(AMM, read), true);
});

test("read error/unavailable → fail-safe (recusa)", async () => {
  const read: IsPausedReader = async () => { throw new Error("rpc down"); };
  assert.equal(await checkNotPaused(AMM, read), false);
});

test("missing/invalid amm_address → fail-safe (recusa)", async () => {
  const read: IsPausedReader = async () => false; // would be "active", but address invalid
  assert.equal(await checkNotPaused(undefined, read), false);
  assert.equal(await checkNotPaused("not-an-address", read), false);
  assert.equal(await checkNotPaused("0x1234", read), false);
});

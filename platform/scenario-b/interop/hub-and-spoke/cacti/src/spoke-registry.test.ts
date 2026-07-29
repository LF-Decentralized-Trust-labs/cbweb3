// SPDX-License-Identifier: Apache-2.0
import { test } from "node:test";
import assert from "node:assert/strict";
import { SpokeRegistry, Spoke } from "./spoke-registry";

const mk = (id: string): Spoke => ({
  spokeId: id,
  besuRpc: `http://host:8545/${id}`,
  besuWs: `ws://host:8546/${id}`,
  gatewayUrl: `http://gw/${id}`,
});

test("registry starts empty (boot neutro)", () => {
  const r = new SpokeRegistry();
  assert.equal(r.size, 0);
  assert.deepEqual(r.list(), []);
});

test("upsert is idempotent by spokeId (no duplicate)", () => {
  const r = new SpokeRegistry();
  r.upsert(mk("spoke-a"));
  r.upsert(mk("spoke-a")); // same id again
  assert.equal(r.size, 1);
  const updated = { ...mk("spoke-a"), gatewayUrl: "http://new" };
  r.upsert(updated);
  assert.equal(r.size, 1);
  assert.equal(r.get("spoke-a")?.gatewayUrl, "http://new");
});

test("get and list reflect N spokes", () => {
  const r = new SpokeRegistry();
  r.hydrate([mk("a"), mk("b"), mk("c")]);
  assert.equal(r.size, 3);
  assert.equal(r.get("b")?.spokeId, "b");
  assert.equal(r.get("missing"), undefined);
  assert.deepEqual(
    r.list().map((s) => s.spokeId).sort(),
    ["a", "b", "c"],
  );
});

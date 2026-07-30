// SPDX-License-Identifier: Apache-2.0
import { test } from "node:test";
import assert from "node:assert/strict";
import { loadSpokesFromEnv, validateSpoke } from "./config";

test("loadSpokesFromEnv returns [] on boot neutro (no SPOKES_JSON)", () => {
  assert.deepEqual(loadSpokesFromEnv({}), []);
  assert.deepEqual(loadSpokesFromEnv({ SPOKES_JSON: "" }), []);
});

test("loadSpokesFromEnv parses N spokes", () => {
  const one = JSON.stringify([
    { spokeId: "spoke-br", besuRpc: "http://h:8545", besuWs: "ws://h:8546", gatewayUrl: "http://gw" },
  ]);
  assert.equal(loadSpokesFromEnv({ SPOKES_JSON: one }).length, 1);

  const three = JSON.stringify(
    ["a", "b", "c"].map((id) => ({
      spokeId: id, besuRpc: "http://h", besuWs: "ws://h", gatewayUrl: "http://gw",
    })),
  );
  assert.equal(loadSpokesFromEnv({ SPOKES_JSON: three }).length, 3);
});

test("validateSpoke rejects missing fields", () => {
  assert.throws(() => validateSpoke({ spokeId: "x" }));
  assert.throws(() => validateSpoke({}));
});

// No fatal SPOKE_A/B_BESU_RPC anymore: absence never throws.
test("no fatal SPOKE_A/B env", () => {
  assert.doesNotThrow(() => loadSpokesFromEnv({}));
});

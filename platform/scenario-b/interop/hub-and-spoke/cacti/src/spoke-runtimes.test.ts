// SPDX-License-Identifier: Apache-2.0
import { test } from "node:test";
import assert from "node:assert/strict";
import { createSpokeRuntimes, RuntimeFactories } from "./spoke-runtimes";
import { Spoke } from "./spoke-registry";

const mk = (id: string): Spoke => ({
  spokeId: id, besuRpc: "http://h", besuWs: "ws://h", gatewayUrl: "http://gw",
});

// SC-001: exactly one connector + one watcher per spoke, for N = 0, 1, 3.
test("createSpokeRuntimes builds one connector/watcher per spoke", async () => {
  for (const n of [0, 1, 3]) {
    let connectors = 0, watchers = 0;
    const f: RuntimeFactories = {
      createConnector: async () => { connectors++; return { connector: {}, stop() {} }; },
      createWatcher: async () => { watchers++; return { watcher: {}, stop() {} }; },
    };
    const spokes = Array.from({ length: n }, (_, i) => mk(`spoke-${i}`));
    const runtimes = await createSpokeRuntimes(spokes, f);
    assert.equal(runtimes.length, n);
    assert.equal(connectors, n);
    assert.equal(watchers, n);
  }
});

test("runtime.stop() stops both connector and watcher", async () => {
  let stops = 0;
  const f: RuntimeFactories = {
    createConnector: async () => ({ connector: {}, stop: () => { stops++; } }),
    createWatcher: async () => ({ watcher: {}, stop: () => { stops++; } }),
  };
  const [rt] = await createSpokeRuntimes([mk("a")], f);
  rt.stop();
  assert.equal(stops, 2);
});

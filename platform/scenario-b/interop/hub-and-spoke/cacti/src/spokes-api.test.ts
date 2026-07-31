// SPDX-License-Identifier: Apache-2.0
import { test } from "node:test";
import assert from "node:assert/strict";
import { promises as fs } from "fs";
import * as os from "os";
import * as path from "path";
import { makeSpokesHandler, ApiResponse } from "./spokes-api";
import { SpokeRegistry } from "./spoke-registry";
import { RelayStore } from "./relay-store";

const validBody = {
  spokeId: "spoke-br", besuRpc: "http://h:8545", besuWs: "ws://h:8546", gatewayUrl: "http://gw",
};

function mockRes(): ApiResponse & { code: number; payload: unknown } {
  const r = {
    code: 0,
    payload: undefined as unknown,
    status(c: number) { r.code = c; return r; },
    json(p: unknown) { r.payload = p; },
  };
  return r;
}

async function newStore(): Promise<RelayStore> {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), "spokes-api-"));
  return new RelayStore(path.join(dir, "spokes.json"));
}

test("valid POST upserts, persists, hydrates, and returns 200", async () => {
  const registry = new SpokeRegistry();
  const store = await newStore();
  let hydrated = 0;
  const handler = makeSpokesHandler({ registry, store, onRegister: () => { hydrated++; } });

  const res = mockRes();
  await handler({ body: validBody }, res);

  assert.equal(res.code, 200);
  assert.equal(registry.size, 1);
  assert.equal(hydrated, 1);
  assert.equal((await store.load()).length, 1);
});

test("re-registering the same spoke is idempotent", async () => {
  const registry = new SpokeRegistry();
  const store = await newStore();
  const handler = makeSpokesHandler({ registry, store, onRegister: () => {} });
  await handler({ body: validBody }, mockRes());
  await handler({ body: validBody }, mockRes());
  assert.equal(registry.size, 1);
});

test("invalid payload → 400 and store unchanged", async () => {
  const registry = new SpokeRegistry();
  const store = await newStore();
  const handler = makeSpokesHandler({ registry, store, onRegister: () => {} });

  const res = mockRes();
  await handler({ body: { spokeId: "x" } }, res); // missing fields

  assert.equal(res.code, 400);
  assert.equal(registry.size, 0);
  assert.deepEqual(await store.load(), []); // never saved
});

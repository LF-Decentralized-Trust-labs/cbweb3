// SPDX-License-Identifier: Apache-2.0
import { test } from "node:test";
import assert from "node:assert/strict";
import type { Request, Response } from "express";
import { CrossCurrencySwapRelay } from "./cross-currency-swap-relay";
import { SpokeRegistry } from "./spoke-registry";

const AUTH = "secret";
const AMM = "0x2222222222222222222222222222222222222222";

function registryWith(...ids: string[]): SpokeRegistry {
  const r = new SpokeRegistry();
  for (const id of ids) {
    r.upsert({ spokeId: id, besuRpc: "http://h", besuWs: "ws://h", gatewayUrl: `http://gw/${id}` });
  }
  return r;
}

function mockRes() {
  const r = { code: 200, payload: undefined as unknown };
  return {
    res: {
      status(c: number) { r.code = c; return this as unknown as Response; },
      json(p: unknown) { r.payload = p; },
    } as unknown as Response,
    out: r,
  };
}

const req = (body: unknown): Request =>
  ({ headers: { "x-relay-auth": AUTH }, body } as unknown as Request);

const okPayload = (spokeOut: string) => ({
  correlation_id: "c1", swap_tx_hash: "0xabc", pool_pair: "W-BRL-ARS",
  amount_out: "100", beneficiary_bank_id: "bank-b", spoke_out: spokeOut,
  wrapped_target_token: "0xtok", amm_address: AMM,
});

// SC-004: routes to the gateway of spoke_out (lookup).
test("resolveGateway returns the registered spoke gateway", () => {
  const relay = new CrossCurrencySwapRelay({
    registry: registryWith("spoke-br"), relayAuthSecret: AUTH, notPaused: async () => true,
  });
  assert.equal(relay.resolveGateway("spoke-br"), "http://gw/spoke-br");
  assert.throws(() => relay.resolveGateway("spoke-unknown"));
});

test("forwards to the resolved gateway (fetch injectável)", async () => {
  let calledUrl = "";
  const relay = new CrossCurrencySwapRelay({
    registry: registryWith("spoke-ar"),
    relayAuthSecret: AUTH,
    notPaused: async () => true,
    fetchFn: (async (url: string) => { calledUrl = url; return new Response("{}", { status: 200 }); }) as typeof fetch,
  });
  const { res, out } = mockRes();
  await relay.handleBridgeOut(req(okPayload("spoke-ar")), res);
  assert.equal(out.code, 200);
  assert.ok(calledUrl.startsWith("http://gw/spoke-ar/"));
});

// SC-004: unknown spoke_out → rejected.
test("unknown spoke_out → 400", async () => {
  const relay = new CrossCurrencySwapRelay({
    registry: registryWith("spoke-br"), relayAuthSecret: AUTH, notPaused: async () => true,
  });
  const { res, out } = mockRes();
  await relay.handleBridgeOut(req(okPayload("spoke-zz")), res);
  assert.equal(out.code, 400);
});

// SC-005: paused pair → 409, no forward.
test("paused pair → 409 (circuit breaker), no forward", async () => {
  let forwarded = false;
  const relay = new CrossCurrencySwapRelay({
    registry: registryWith("spoke-br"),
    relayAuthSecret: AUTH,
    notPaused: async () => false, // paused / fail-safe
    fetchFn: (async () => { forwarded = true; return new Response("{}", { status: 200 }); }) as typeof fetch,
  });
  const { res, out } = mockRes();
  await relay.handleBridgeOut(req(okPayload("spoke-br")), res);
  assert.equal(out.code, 409);
  assert.equal(forwarded, false);
});

test("bad auth → 401", async () => {
  const relay = new CrossCurrencySwapRelay({
    registry: registryWith("spoke-br"), relayAuthSecret: AUTH, notPaused: async () => true,
  });
  const { res, out } = mockRes();
  const badReq = { headers: { "x-relay-auth": "wrong" }, body: okPayload("spoke-br") } as unknown as Request;
  await relay.handleBridgeOut(badReq, res);
  assert.equal(out.code, 401);
});

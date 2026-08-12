// SPDX-License-Identifier: Apache-2.0
import { test } from "node:test";
import assert from "node:assert/strict";
import crypto from "node:crypto";
import type { Request, Response } from "express";
import { CrossCurrencySwapRelay } from "./cross-currency-swap-relay";
import { SpokeRegistry } from "./spoke-registry";
import { RelaySigner, canonicalString } from "./relay-auth";

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

test("forwards signed with the relay's own identity, over the exact body sent", async () => {
  const { privateKey, publicKey } = crypto.generateKeyPairSync("ec", { namedCurve: "prime256v1" });
  const signer = new RelaySigner(
    "cacti-relay",
    privateKey.export({ type: "sec1", format: "pem" }).toString(),
  );

  let seenBody = "";
  let seenHeaders: Record<string, string> = {};
  const relay = new CrossCurrencySwapRelay({
    registry: registryWith("spoke-ar"),
    relayAuthSecret: AUTH,
    notPaused: async () => true,
    signer,
    fetchFn: (async (_url: string, init: any) => {
      seenBody = init.body;
      seenHeaders = init.headers;
      return new Response("{}", { status: 200 });
    }) as typeof fetch,
  });

  const { res, out } = mockRes();
  await relay.handleBridgeOut(req(okPayload("spoke-ar")), res);
  assert.equal(out.code, 200);

  // The signature must verify over the body the RECEIVER gets. Signing a separately serialized copy
  // is the mistake this asserts against: it type-checks, and rejects every request in production.
  const ok = crypto
    .createVerify("SHA256")
    .update(
      canonicalString(
        Number(seenHeaders["X-Relay-Timestamp"]),
        "POST",
        "/internal/amm/cross-currency-bridge-out",
        seenBody,
      ),
    )
    .verify(publicKey, Buffer.from(seenHeaders["X-Relay-Signature"], "base64"));
  assert.equal(ok, true, "the forwarded request's signature does not verify over its own body");
  assert.equal(seenHeaders["X-Relay-Key-Id"], "cacti-relay");
  // The shared secret survives the migration window, so a CB that has not pinned the relay yet does
  // not fail the bridge-out leg closed.
  assert.equal(seenHeaders["X-Relay-Auth"], AUTH);
});

// Without a signer the request must carry no signature headers at all — half-signed requests are
// accepted by a CB that has not pinned the relay and rejected by one that has.
test("unsigned when no signer is configured", async () => {
  let seenHeaders: Record<string, string> = {};
  const relay = new CrossCurrencySwapRelay({
    registry: registryWith("spoke-ar"),
    relayAuthSecret: AUTH,
    notPaused: async () => true,
    fetchFn: (async (_url: string, init: any) => {
      seenHeaders = init.headers;
      return new Response("{}", { status: 200 });
    }) as typeof fetch,
  });
  const { res } = mockRes();
  await relay.handleBridgeOut(req(okPayload("spoke-ar")), res);
  assert.equal(seenHeaders["X-Relay-Key-Id"], undefined);
  assert.equal(seenHeaders["X-Relay-Auth"], AUTH);
});

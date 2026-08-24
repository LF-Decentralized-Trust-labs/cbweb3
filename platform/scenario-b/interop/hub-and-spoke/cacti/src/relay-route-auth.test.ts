// SPDX-License-Identifier: Apache-2.0

/**
 * Tests for the relay's inbound route guard (finding R2-M-10).
 *
 * Two defects motivate this module. POST /api/v1/spokes accepted a registration from any
 * caller that could reach the port: registering a spoke points the relay at a chosen Besu
 * RPC and contract set, so an unauthenticated write there redirects what the relay watches
 * and where it forwards settlement. And the bridge-out route compared its secret with
 * `!==`, which returns as soon as two bytes differ and so leaks, through response timing,
 * how many leading bytes a guess got right.
 *
 * A unit test cannot assert constant time — timing is a property of the comparison, not of
 * an observable result. What these tests pin instead is that there is exactly ONE
 * comparison path (`isRelayAuthorized`) for every inbound route, that it fails closed, and
 * that it never throws on adversarial input. `timingSafeEqual` throws on length-mismatched
 * buffers, so an unguarded port of the scenario-a pattern would turn a wrong guess into a
 * 500 and a crash surface.
 */

import { test } from "node:test";
import assert from "node:assert/strict";
import { isRelayAuthorized, requireRelayAuth } from "./relay-route-auth";

const SECRET = "cbweb3-relay-shared-secret";

test("accepts the exact secret", () => {
  assert.equal(isRelayAuthorized(SECRET, SECRET), true);
});

test("rejects a missing header", () => {
  assert.equal(isRelayAuthorized(undefined, SECRET), false);
  assert.equal(isRelayAuthorized("", SECRET), false);
});

test("rejects a wrong secret of the same length", () => {
  const wrong = "x".repeat(SECRET.length);
  assert.equal(wrong.length, SECRET.length);
  assert.equal(isRelayAuthorized(wrong, SECRET), false);
});

test("rejects a wrong secret of a different length without throwing", () => {
  // timingSafeEqual throws RangeError on unequal lengths; the guard must absorb that.
  assert.equal(isRelayAuthorized("short", SECRET), false);
  assert.equal(isRelayAuthorized(SECRET + "extra", SECRET), false);
});

test("rejects a secret that only shares a prefix", () => {
  assert.equal(isRelayAuthorized(SECRET.slice(0, -1) + "!", SECRET), false);
});

test("fails closed when no secret is configured", () => {
  // An unset INTERNAL_RELAY_AUTH_SECRET must never mean "allow everything".
  assert.equal(isRelayAuthorized("anything", ""), false);
  assert.equal(isRelayAuthorized("", ""), false);
});

test("rejects a non-string header without coercing it", () => {
  // express hands back string[] for a repeated header; neither form may authenticate.
  assert.equal(isRelayAuthorized([SECRET] as unknown as string, SECRET), false);
  assert.equal(isRelayAuthorized({ toString: () => SECRET } as unknown as string, SECRET), false);
});

test("middleware passes an authenticated request through", () => {
  const guard = requireRelayAuth(SECRET);
  let nexted = 0;
  let status = 0;
  const res = {
    status(c: number) { status = c; return res; },
    json() { /* not expected */ },
  };
  guard(
    { headers: { "x-relay-auth": SECRET } } as never,
    res as never,
    () => { nexted++; },
  );
  assert.equal(nexted, 1);
  assert.equal(status, 0, "an authenticated request must not receive a status");
});

test("middleware rejects an unauthenticated request with 401 and does not call next", () => {
  const guard = requireRelayAuth(SECRET);
  let nexted = 0;
  let status = 0;
  let payload: unknown;
  const res = {
    status(c: number) { status = c; return res; },
    json(p: unknown) { payload = p; },
  };
  guard({ headers: {} } as never, res as never, () => { nexted++; });
  assert.equal(status, 401);
  assert.equal(nexted, 0, "a rejected request must never reach the handler");
  assert.deepEqual(payload, { error: "X-Relay-Auth invalid or missing" });
});

test("middleware built with no secret rejects every request", () => {
  const guard = requireRelayAuth("");
  let nexted = 0;
  let status = 0;
  const res = { status(c: number) { status = c; return res; }, json() {} };
  guard({ headers: { "x-relay-auth": "anything" } } as never, res as never, () => { nexted++; });
  assert.equal(status, 401);
  assert.equal(nexted, 0);
});

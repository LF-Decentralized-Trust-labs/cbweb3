import { strict as assert } from "node:assert";
import test from "node:test";
import crypto from "node:crypto";
import { canonicalString, RelaySigner } from "./relay-auth";

// The relay is the last caller on the internal endpoints that authenticates with the shared
// symmetric secret — the one identical in every entity. Until it signs, RELAY_REQUIRE_SIGNATURE
// cannot be turned on without rejecting the bridge-out leg.
//
// The relay signs with its OWN identity rather than forwarding the originating bank's: it
// re-serializes the payload before forwarding (JSON.stringify of a parsed object), so the bytes it
// sends are not the bytes it received and a forwarded signature would never verify. Nothing is lost
// by that, because the destination CB verifies the swap ON-CHAIN from its receipt (LogSwap from the
// pair's AMM, amounts read from the event, each swap_tx_hash consumed once) — the act is attributed
// cryptographically on the ledger, so the transport credential only has to authenticate the hop.
//
// The canonical string is the one thing that MUST agree byte-for-byte with the Go verifier. A
// mismatch does not degrade: every request is rejected. The fixture below was produced by the Go
// implementation, so this test fails if either side drifts.

const GO_FIXTURE_BODY = '{"correlation_id":"abc","spoke_out":"spoke-ars"}';
const GO_FIXTURE_CANONICAL =
  "cbweb3-relay-v1\n" +
  "1700000000\n" +
  "POST\n" +
  "/internal/amm/cross-currency-bridge-out\n" +
  "31249686edc07596e475947c424f2751de4a6d9374fe01f41fd1d5294197f355";

test("canonicalString matches the Go implementation byte for byte", () => {
  const got = canonicalString(
    1700000000,
    "POST",
    "/internal/amm/cross-currency-bridge-out",
    GO_FIXTURE_BODY,
  );
  assert.equal(got, GO_FIXTURE_CANONICAL);
});

test("canonicalString upper-cases the method, as the verifier does", () => {
  const lower = canonicalString(1700000000, "post", "/x", "");
  const upper = canonicalString(1700000000, "POST", "/x", "");
  assert.equal(lower, upper);
});

// A signature bound to a different path or body must not verify: that binding is what makes a
// captured signature non-transferable to another request.
test("the signature is bound to method, path and body", () => {
  const { privateKey, publicKey } = crypto.generateKeyPairSync("ec", {
    namedCurve: "prime256v1",
  });
  const keyPem = privateKey.export({ type: "sec1", format: "pem" }).toString();
  const signer = new RelaySigner("cacti-relay", keyPem);

  const body = '{"a":1}';
  const path = "/internal/amm/cross-currency-bridge-out";
  const headers = signer.headersFor("POST", path, body);

  const verify = (m: string, p: string, b: string) =>
    crypto
      .createVerify("SHA256")
      .update(canonicalString(Number(headers["X-Relay-Timestamp"]), m, p, b))
      .verify(publicKey, Buffer.from(headers["X-Relay-Signature"], "base64"));

  assert.equal(verify("POST", path, body), true, "the signature must verify over what was signed");
  assert.equal(verify("POST", path, '{"a":2}'), false, "a different body must not verify");
  assert.equal(verify("POST", "/internal/amm/other", body), false, "a different path must not verify");
  assert.equal(verify("GET", path, body), false, "a different method must not verify");
});

test("headersFor emits the three headers the verifier reads", () => {
  const { privateKey } = crypto.generateKeyPairSync("ec", { namedCurve: "prime256v1" });
  const signer = new RelaySigner(
    "cacti-relay",
    privateKey.export({ type: "sec1", format: "pem" }).toString(),
  );
  const h = signer.headersFor("POST", "/x", "{}");
  assert.equal(h["X-Relay-Key-Id"], "cacti-relay");
  assert.ok(/^\d+$/.test(h["X-Relay-Timestamp"]), "timestamp must be decimal unix seconds");
  assert.ok(h["X-Relay-Signature"].length > 0);
});

// A relay with no key must not pretend to sign: half-signed requests would be rejected by a
// verifier that has the relay pinned, and silently accepted by one that does not.
test("no key means no signature headers, not empty ones", () => {
  const signer = RelaySigner.fromOptional("cacti-relay", undefined);
  assert.equal(signer, undefined);
});

// SPDX-License-Identifier: Apache-2.0

/**
 * Inbound authentication for the relay's own HTTP routes (finding R2-M-10).
 *
 * WHY THIS EXISTS AS ONE MODULE. The relay had two inbound routes with two different
 * stories. `POST /api/v1/spokes` had no credential at all, so any caller that could reach
 * the port could register a spoke — and a spoke registration is a pointer to a Besu RPC and
 * a gateway URL, which is to say a pointer to what the relay watches and where it forwards
 * settlement. `POST /api/v1/cross-currency/bridge-out` did check the shared secret, but with
 * `provided !== secret`: a comparison that short-circuits on the first differing byte and so
 * discloses, through response timing, how much of a guess was correct. Funnelling both
 * through one audited comparison is the point of this module — a second implementation is
 * how the first one silently drifts.
 *
 * WHAT THIS IS NOT. The shared secret is identical in every entity of a deployment, so it
 * authenticates the hop and nothing more: it cannot attribute a call to a particular bank or
 * central bank. Per-entity attribution is the relay's outbound direction (see relay-auth.ts,
 * where the relay signs with its own key) and, for inbound, remains open under §14.D. This
 * module closes an anonymous-write hole; it does not make the route multi-tenant.
 */

import { timingSafeEqual } from "node:crypto";
import type { NextFunction, Request, Response } from "express";

/** The header every inbound relay route authenticates with. */
export const HEADER_RELAY_AUTH = "x-relay-auth";

/**
 * isRelayAuthorized compares a presented credential against the configured secret in
 * constant time relative to the secret's content.
 *
 * Three deliberate properties:
 *
 * 1. **Fails closed on an unconfigured secret.** An empty `secret` authorizes nothing. The
 *    alternative — treating "no secret" as "no check" — is how a misconfigured deployment
 *    silently serves an open endpoint.
 * 2. **Never throws.** `timingSafeEqual` raises RangeError on length-mismatched buffers, so
 *    the length is compared first. A guard that throws converts a wrong guess into a 500 and
 *    hands the caller a crash surface instead of a rejection.
 * 3. **Does not coerce.** express yields `string[]` for a repeated header; only a real string
 *    is considered, so no exotic input can stringify its way into a match.
 *
 * The length check does reveal the secret's length. That is accepted: the length of a shared
 * credential is not the credential, and the alternative (hashing both sides to a fixed width)
 * buys nothing an attacker could not already infer from the deployment's own configuration.
 */
export function isRelayAuthorized(provided: unknown, secret: string): boolean {
  if (typeof secret !== "string" || secret.length === 0) return false;
  if (typeof provided !== "string" || provided.length === 0) return false;
  const presented = Buffer.from(provided, "utf8");
  const expected = Buffer.from(secret, "utf8");
  if (presented.length !== expected.length) return false;
  return timingSafeEqual(presented, expected);
}

/**
 * requireRelayAuth builds the express guard for an inbound relay route. Rejection is a flat
 * 401 with no detail about which part failed — a guard that distinguishes "missing" from
 * "wrong" is an oracle.
 */
export function requireRelayAuth(secret: string) {
  return (req: Request, res: Response, next: NextFunction): void => {
    if (!isRelayAuthorized(req.headers[HEADER_RELAY_AUTH], secret)) {
      res.status(401).json({ error: "X-Relay-Auth invalid or missing" });
      return;
    }
    next();
  };
}

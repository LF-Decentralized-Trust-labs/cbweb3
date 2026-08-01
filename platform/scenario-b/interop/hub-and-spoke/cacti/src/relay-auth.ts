import crypto from "node:crypto";
import fs from "node:fs";

/**
 * Per-entity signing for the relay's calls to a central bank's internal endpoints.
 *
 * WHY THE RELAY SIGNS AT ALL. Those endpoints accepted a shared symmetric secret — identical in
 * every entity, so any entity could forge a call as any other. The relay was the last caller still
 * on it, which is what blocked turning RELAY_REQUIRE_SIGNATURE on: enforcement without a signing
 * relay rejects the bridge-out leg and stops settlement.
 *
 * WHY ITS OWN IDENTITY, not the originating bank's forwarded. The relay re-serializes the payload
 * before forwarding (JSON.stringify of a parsed object), so the bytes it sends are not the bytes it
 * received and a forwarded signature could never verify over them. Nothing important is lost:
 * the destination central bank verifies the swap ON-CHAIN from its receipt — LogSwap emitted by the
 * pair's own AMM, amounts read from the event rather than the payload, and each swap_tx_hash
 * consumed at most once. The act is attributed cryptographically on the ledger, so this credential
 * only has to authenticate the hop.
 *
 * The canonical string must agree byte-for-byte with the Go verifier
 * (backend/services/api-gateway/internal/relayauth). A mismatch does not degrade gracefully — every
 * request is rejected — so relay-auth.test.ts pins it against a fixture produced by that Go code.
 */

const SCHEME = "cbweb3-relay-v1";

export const HEADER_KEY_ID = "X-Relay-Key-Id";
export const HEADER_TIMESTAMP = "X-Relay-Timestamp";
export const HEADER_SIGNATURE = "X-Relay-Signature";

/**
 * canonicalString builds the exact bytes that are signed and verified: scheme, timestamp, method,
 * path and the hex SHA-256 of the body, newline-separated. Binding all of them makes a captured
 * signature non-transferable to a different request and non-replayable outside the verifier's skew
 * window.
 */
export function canonicalString(
  timestampSeconds: number,
  method: string,
  path: string,
  body: string,
): string {
  const bodyHash = crypto.createHash("sha256").update(body, "utf8").digest("hex");
  return [SCHEME, String(timestampSeconds), method.toUpperCase(), path, bodyHash].join("\n");
}

export class RelaySigner {
  constructor(
    private readonly keyId: string,
    private readonly privateKeyPem: string,
  ) {}

  /**
   * fromOptional loads a signer from a PEM key file, returning undefined when no path is configured
   * or the file cannot be read.
   *
   * Undefined rather than a no-op signer on purpose: a signer that emits no headers looks like a
   * working one, and the resulting requests are silently accepted by a CB that has not pinned the
   * relay and rejected by one that has. The caller logs the absence and falls back to the shared
   * secret explicitly.
   */
  static fromOptional(keyId: string, keyPath: string | undefined): RelaySigner | undefined {
    if (!keyPath) return undefined;
    try {
      const pem = fs.readFileSync(keyPath, "utf8");
      if (!pem.includes("PRIVATE KEY")) {
        console.error(
          `[relay-auth] ${keyPath} does not contain a PEM private key — requests will not be signed`,
        );
        return undefined;
      }
      return new RelaySigner(keyId, pem);
    } catch (err) {
      console.error(
        `[relay-auth] could not read the signing key ${keyPath}: ${(err as Error).message} — requests will not be signed`,
      );
      return undefined;
    }
  }

  /**
   * headersFor signs one request. `body` must be the exact string sent as the request body: the
   * signature covers its bytes, so serializing twice (once to sign, once to send) is how a subtle
   * mismatch gets introduced.
   */
  headersFor(method: string, path: string, body: string): Record<string, string> {
    const ts = Math.floor(Date.now() / 1000);
    const signature = crypto
      .createSign("SHA256")
      .update(canonicalString(ts, method, path, body))
      .sign(this.privateKeyPem)
      .toString("base64");
    return {
      [HEADER_KEY_ID]: this.keyId,
      [HEADER_TIMESTAMP]: String(ts),
      [HEADER_SIGNATURE]: signature,
    };
  }
}

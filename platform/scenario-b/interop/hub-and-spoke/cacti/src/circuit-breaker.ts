// SPDX-License-Identifier: Apache-2.0

/**
 * Circuit-breaker validation (TK-B5, Constitution III). Before forwarding a
 * swap/bridge-out, the relay reads isPaused() ON-CHAIN on the pair's AMM
 * (address from the payload's amm_address). Fail-safe: a missing/invalid address
 * or an unavailable read is treated as PAUSED (do not forward).
 */

import { ethers } from "ethers";

const AMM_ABI = ["function isPaused() view returns (bool)"];

/** Reads isPaused() for an AMM address. Injectable for tests. */
export type IsPausedReader = (ammAddress: string) => Promise<boolean>;

/** Production reader: queries isPaused() on-chain via ethers v6. */
export function makeEthersIsPausedReader(rpcUrl: string): IsPausedReader {
  const provider = new ethers.JsonRpcProvider(rpcUrl);
  return async (ammAddress: string): Promise<boolean> => {
    const amm = new ethers.Contract(ammAddress, AMM_ABI, provider);
    return (await amm.isPaused()) as boolean;
  };
}

const ADDRESS_RE = /^0x[0-9a-fA-F]{40}$/;

/** Minimal structured logger — the relay logs JSON to stdout (defaults to console). */
export type CircuitBreakerLogger = Pick<Console, "warn">;

/** Why the breaker refused to forward — distinguishes a genuine pause from a config/RPC fault. */
export type RefuseReason = "missing_address" | "paused" | "read_error";

function logRefusal(log: CircuitBreakerLogger, reason: RefuseReason, ammAddress: string | undefined, error?: unknown): void {
  log.warn(JSON.stringify({
    ts: new Date().toISOString(),
    service: "cacti-relay",
    severity: "WARN",
    event: "circuit_breaker_refused",
    reason,
    amm_address: ammAddress ?? null,
    // A read_error is NOT a pause — it means the isPaused() call itself failed (e.g. the Hub RPC
    // is unreachable or misconfigured). Surfacing the message prevents a silent fail-safe from
    // masquerading as "pair paused" (constitution: no silent error swallowing).
    ...(error !== undefined ? { error: String(error) } : {}),
  }));
}

/**
 * Returns true only when it is SAFE to forward (AMM exists and is NOT paused).
 * Missing/invalid amm_address, a paused AMM, or a failed read → false (fail-safe).
 *
 * The three refusal cases are logged with a distinct `reason` (and, for a read fault, the
 * underlying error) so operators can tell an actual circuit-breaker trip apart from a
 * config/connectivity fault. The fail-safe behaviour is unchanged — only observability improves.
 */
export async function checkNotPaused(
  ammAddress: string | undefined,
  read: IsPausedReader,
  log: CircuitBreakerLogger = console,
): Promise<boolean> {
  if (!ammAddress || !ADDRESS_RE.test(ammAddress)) {
    logRefusal(log, "missing_address", ammAddress);
    return false; // fail-safe
  }
  try {
    const paused = await read(ammAddress);
    if (paused) {
      logRefusal(log, "paused", ammAddress);
      return false;
    }
    return true;
  } catch (err) {
    logRefusal(log, "read_error", ammAddress, err);
    return false; // fail-safe: unavailable read ⇒ do not forward
  }
}

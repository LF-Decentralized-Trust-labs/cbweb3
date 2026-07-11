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

/**
 * Returns true only when it is SAFE to forward (AMM exists and is NOT paused).
 * Missing/invalid amm_address, a paused AMM, or a failed read → false (fail-safe).
 */
export async function checkNotPaused(
  ammAddress: string | undefined,
  read: IsPausedReader,
): Promise<boolean> {
  if (!ammAddress || !ADDRESS_RE.test(ammAddress)) return false; // fail-safe
  try {
    return !(await read(ammAddress));
  } catch {
    return false; // fail-safe: unavailable read ⇒ do not forward
  }
}

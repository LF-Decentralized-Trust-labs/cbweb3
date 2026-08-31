// SPDX-License-Identifier: Apache-2.0

/**
 * Formats a token amount for display without going through Number.
 *
 * The treasury screens used to render `Number(value).toLocaleString()` over a
 * bigint (finding R2-M-8). A double holds only 53 bits of integer precision, so
 * any balance above 9_007_199_254_740_991 was silently rounded — and these are
 * exactly the figures an operator reads before deciding how much money to
 * destroy. bigint carries its own toLocaleString, which is exact at any size.
 */
export const formatTokenAmount = (value: bigint): string => value.toLocaleString("en-US");

/**
 * Formats an amount that arrives from the API as a decimal string.
 *
 * Same defect, other shape: the dashboard, the funding queue and the
 * reconciliation view all ran `Number(value).toLocaleString()`, which rounds
 * once the figure passes 2^53 — and reconciliation exists precisely to show
 * whether two ledgers agree, so a rounded delta can hide a real mismatch.
 *
 * Falls back to the raw string rather than throwing: an amount that is not a
 * plain integer (empty, fractional, or malformed) is shown as received instead
 * of crashing the screen or silently displaying a wrong number.
 */
export const formatAmountString = (value: string): string => {
  const trimmed = value?.trim() ?? "";
  if (!/^-?\d+$/.test(trimmed)) {
    return value;
  }
  return formatTokenAmount(BigInt(trimmed));
};

// SPDX-License-Identifier: Apache-2.0

// Shared formatting helpers for the cooperative-liquidity UI.

// formatRemainingMs renders a millisecond duration as "Hh Mm Ss", or "Expired" at/below zero.
export function formatRemainingMs(ms: number): string {
  if (ms <= 0) {
    return "Expired";
  }
  const totalSeconds = Math.floor(ms / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  return `${hours}h ${minutes}m ${seconds}s`;
}

// remainingMsUntil returns the ms remaining until an ISO timestamp, clamped at 0.
// Returns 0 for missing or unparseable input.
export function remainingMsUntil(isoTimestamp: string | undefined | null, nowMs: number): number {
  if (!isoTimestamp) {
    return 0;
  }
  const expiryMs = Date.parse(isoTimestamp);
  if (Number.isNaN(expiryMs)) {
    return 0;
  }
  return Math.max(0, expiryMs - nowMs);
}

// poolPairCurrencies extracts the two side currency codes from a pool-pair id.
// The pair is a cross-currency hub pool whose tokens are not on this spoke chain, so the
// configured pair id is the source of truth for the side labels (not an on-chain symbol).
// Accepts "W-BRL-ARS" (wrapped) or "BRL-ARS" → { a: "BRL", b: "ARS" }. Falls back to the
// generic "A"/"B" placeholders when the id can't be parsed.
export function poolPairCurrencies(pair: string | null | undefined): { a: string; b: string } {
  if (pair) {
    let s = pair.trim();
    if (/^w-/i.test(s)) {
      s = s.slice(2);
    }
    const parts = s.split("-").filter(Boolean);
    if (parts.length >= 2) {
      return { a: parts[0], b: parts[1] };
    }
  }
  return { a: "A", b: "B" };
}

// PoolSideInfo describes one side of a pool relative to the viewing Central Bank.
// `role` is "national" when the side currency matches the CB's own (on-chain) currency,
// "foreign" when it doesn't, or null when the national currency is not yet known.
export interface PoolSideInfo {
  currency: string;
  role: "national" | "foreign" | null;
}

// classifyPoolSides labels each side (A/B) of a pool against the viewing CB's national
// currency. The national currency is sourced on-chain from the CB's own tCeBM symbol; the
// foreign side comes from the pair id (env today; the Hub will expose multiple pairs later).
// When nationalCurrency is unknown, both roles are null and callers show a neutral fallback.
export function classifyPoolSides(
  pair: string | null | undefined,
  nationalCurrency: string | null | undefined,
): { a: PoolSideInfo; b: PoolSideInfo } {
  const { a, b } = poolPairCurrencies(pair);
  const nat = (nationalCurrency ?? "").trim().toUpperCase();
  const roleOf = (currency: string): "national" | "foreign" | null => {
    if (!nat) {
      return null;
    }
    return currency.toUpperCase() === nat ? "national" : "foreign";
  };
  return { a: { currency: a, role: roleOf(a) }, b: { currency: b, role: roleOf(b) } };
}

// poolSideInfo returns the classification for a specific side letter ("A"/"B").
export function poolSideInfo(
  side: string | null | undefined,
  pair: string | null | undefined,
  nationalCurrency: string | null | undefined,
): PoolSideInfo | null {
  const sides = classifyPoolSides(pair, nationalCurrency);
  const normalized = (side ?? "").trim().toUpperCase();
  if (normalized === "A") {
    return sides.a;
  }
  if (normalized === "B") {
    return sides.b;
  }
  return null;
}

// sideRoleLabel renders a side as "National Currency (BRL)" / "Foreign Currency (ARS)".
// `short` produces "National (BRL)" / "Foreign (ARS)". When the role is unknown it falls back
// to `${fallbackLabel} (CUR)` (e.g. "Token A (BRL)"), or just the currency if no fallback.
export function sideRoleLabel(
  info: PoolSideInfo | null,
  opts: { short?: boolean; fallbackLabel?: string } = {},
): string {
  if (!info) {
    return "";
  }
  const { short = false, fallbackLabel } = opts;
  let text: string;
  if (info.role === "national") {
    text = short ? "National" : "National Currency";
  } else if (info.role === "foreign") {
    text = short ? "Foreign" : "Foreign Currency";
  } else {
    text = fallbackLabel ?? "";
  }
  return text ? `${text} (${info.currency})` : info.currency;
}

// truncateAddress shortens an EVM address to "0x1234…abcd" for display.
export function truncateAddress(address: string): string {
  if (address.length <= 12) {
    return address;
  }
  return `${address.slice(0, 6)}…${address.slice(-4)}`;
}

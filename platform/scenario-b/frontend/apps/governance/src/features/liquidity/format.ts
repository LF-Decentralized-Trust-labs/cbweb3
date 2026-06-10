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

// truncateAddress shortens an EVM address to "0x1234…abcd" for display.
export function truncateAddress(address: string): string {
  if (address.length <= 12) {
    return address;
  }
  return `${address.slice(0, 6)}…${address.slice(-4)}`;
}

// SPDX-License-Identifier: Apache-2.0

// Data freshness for the status bar. The portal has no telemetry stream (see
// AppLayout) — every screen is HTTP polling — so "live vs stale" is decided by how
// long it has been since a backend call last succeeded, not by a socket's state.

import { create } from "zustand";

type FreshnessState = {
  lastSuccessAt: number | null;
  lastErrorAt: number | null;
  markSuccess: () => void;
  markError: () => void;
};

export const useFreshnessStore = create<FreshnessState>((set) => ({
  lastSuccessAt: null,
  lastErrorAt: null,
  markSuccess: () => set({ lastSuccessAt: Date.now() }),
  markError: () => set({ lastErrorAt: Date.now() }),
}));

/** Grace factor applied to the polling interval before data counts as stale. */
export const STALE_INTERVAL_FACTOR = 3;
/** Floor for the stale window, so a 5s interval does not flip on a single slow reply. */
export const STALE_MIN_SECONDS = 20;

/**
 * Whether the screen is showing data the operator should not trust: more than
 * STALE_INTERVAL_FACTOR polling cycles (never less than STALE_MIN_SECONDS) without a
 * successful call. With no success yet, a failed call already counts as stale — a
 * portal that never reached the backend must not claim to be live — while the moments
 * before the very first reply are neither live nor stale.
 */
export function isDataStale(
  lastSuccessAt: number | null,
  nowMs: number,
  pollingSeconds: number,
  lastErrorAt: number | null = null,
): boolean {
  if (lastSuccessAt === null) return lastErrorAt !== null;
  const windowSeconds = Math.max(pollingSeconds * STALE_INTERVAL_FACTOR, STALE_MIN_SECONDS);
  return nowMs - lastSuccessAt > windowSeconds * 1000;
}

/**
 * Container logs are agent-pushed snapshots, not a live tail: when a container stops (or
 * its agent does), the viewer keeps serving the last snapshot with no visible change.
 * Snapshots older than this are flagged in the viewer, which itself reloads every 15s.
 */
export const LOG_STALE_SECONDS = 60;

/** Age in seconds of the newest log line, or null when there is nothing to date. */
export function snapshotAgeSeconds(newestOccurredAt: string | undefined, nowMs: number): number | null {
  if (!newestOccurredAt) return null;
  const collectedAt = Date.parse(newestOccurredAt);
  if (Number.isNaN(collectedAt)) return null;
  return Math.max(0, Math.round((nowMs - collectedAt) / 1000));
}
